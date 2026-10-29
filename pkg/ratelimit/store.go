// Package ratelimit supplies shared, atomic request quotas. Keys contain HMACs,
// never raw email addresses, tokens or client identities.
package ratelimit

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// A Rule is either a continuously refilling bucket or an exact rolling window.
// Names are application constants; identities are bounded and hashed before use.
type Rule struct {
	Name, Identity string
	Limit          int
	Window         time.Duration
	Burst          int
}

func Bucket(name, identity string, perMinute, burst int) Rule {
	return Rule{name, identity, perMinute, time.Minute, burst}
}
func Window(name, identity string, limit int, period time.Duration) Rule {
	return Rule{name, identity, limit, period, 0}
}

type Decision struct {
	Allowed     bool
	RetryAfter  int
	Reservation string
}

// Read all quotas before spending any of them. Redis TIME avoids clock skew
// between replicas. Rolling windows are bounded by their small policy limits.
const script = `
local stamp=redis.call('TIME')
local now=tonumber(stamp[1])*1000+math.floor(tonumber(stamp[2])/1000)
local wait=0
local states={}
for i,key in ipairs(KEYS) do
 local offset=(i-1)*3
 local limit=tonumber(ARGV[offset+1])
 local period=tonumber(ARGV[offset+2])
 local burst=tonumber(ARGV[offset+3])
 if burst>0 then
  local old=redis.call('HMGET',key,'tokens','time')
  local tokens=tonumber(old[1]) or burst
  local last=tonumber(old[2]) or now
  tokens=math.min(burst,tokens+math.max(0,now-last)*limit/period)
  states[i]=tokens
  if tokens<1 then wait=math.max(wait,math.ceil((1-tokens)*period/limit)) end
 else
  redis.call('ZREMRANGEBYSCORE',key,'-inf',now-period)
  if redis.call('ZCARD',key)>=limit then
   local first=redis.call('ZRANGE',key,0,0,'WITHSCORES')
   wait=math.max(wait,tonumber(first[2])+period-now)
  end
 end
end
if wait>0 then return {0,math.max(1,math.ceil(wait/1000))} end
local reservation=ARGV[#KEYS*3+1]
for i,key in ipairs(KEYS) do
 local offset=(i-1)*3
 local limit=tonumber(ARGV[offset+1])
 local period=tonumber(ARGV[offset+2])
 local burst=tonumber(ARGV[offset+3])
 if burst>0 then
  redis.call('HSET',key,'tokens',states[i]-1,'time',now)
  redis.call('PEXPIRE',key,math.ceil(period*burst/limit)+1000)
 else
  redis.call('ZADD',key,now,reservation)
  redis.call('PEXPIRE',key,period+1000)
 end
end
return {1,0}
`

type Store struct {
	Client    *redis.Client
	Namespace string
	Secret    []byte
}

func (s *Store) key(r Rule) string {
	h := hmac.New(sha256.New, s.Secret)
	h.Write([]byte(r.Identity))
	return "ratelimit:" + s.Namespace + ":" + r.Name + ":" + hex.EncodeToString(h.Sum(nil))
}
func valid(r Rule) bool {
	return r.Name != "" && len(r.Name) <= 64 && len(r.Identity) <= 1024 && r.Limit > 0 && r.Window > 0 && r.Window <= 24*time.Hour && r.Burst >= 0
}
func (s *Store) Check(ctx context.Context, rules ...Rule) (Decision, error) {
	if len(rules) == 0 || len(rules) > 16 {
		return Decision{}, errors.New("invalid quota count")
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return Decision{}, err
	}
	id := hex.EncodeToString(nonce[:])
	keys := make([]string, 0, len(rules))
	args := make([]any, 0, len(rules)*3+1)
	for _, r := range rules {
		if !valid(r) {
			return Decision{}, errors.New("invalid quota policy")
		}
		keys = append(keys, s.key(r))
		args = append(args, r.Limit, r.Window.Milliseconds(), r.Burst)
	}
	args = append(args, id)
	ctx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()
	result, err := s.Client.Eval(ctx, script, keys, args...).Int64Slice()
	if err != nil {
		return Decision{}, fmt.Errorf("quota store unavailable: %w", err)
	}
	if len(result) != 2 {
		return Decision{}, errors.New("invalid quota response")
	}
	return Decision{Allowed: result[0] == 1, RetryAfter: int(result[1]), Reservation: id}, nil
}

// Release removes a successful login's temporary failure reservation. Other
// login attempts remain counted; success never resets an IP or account quota.
func (s *Store) Release(ctx context.Context, rule Rule, reservation string) error {
	ctx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()
	return s.Client.ZRem(ctx, s.key(rule), reservation).Err()
}

// Fallback bounds ordinary traffic during store outages. It intentionally has
// weaker per-process guarantees; security-sensitive operations never use it.
type localState struct {
	tokens  float64
	updated time.Time
}
type fallback struct {
	sync.Mutex
	entries map[string]localState
	now     func() time.Time
}

func (f *fallback) check(key string, r Rule) Decision {
	f.Lock()
	defer f.Unlock()
	now := f.now()
	if f.entries == nil {
		f.entries = make(map[string]localState)
	}
	state, found := f.entries[key]
	if !found {
		if len(f.entries) >= 10000 {
			for k, v := range f.entries {
				if now.Sub(v.updated) > time.Minute {
					delete(f.entries, k)
				}
			}
		}
		if len(f.entries) >= 10000 {
			return Decision{RetryAfter: 60}
		}
		state = localState{float64(r.Burst), now}
	}
	state.tokens = math.Min(float64(r.Burst), state.tokens+now.Sub(state.updated).Seconds()*float64(r.Limit)/r.Window.Seconds())
	state.updated = now
	d := Decision{Allowed: state.tokens >= 1}
	if d.Allowed {
		state.tokens--
	} else {
		d.RetryAfter = int(math.Ceil((1 - state.tokens) * r.Window.Seconds() / float64(r.Limit)))
		if d.RetryAfter < 1 {
			d.RetryAfter = 1
		}
	}
	f.entries[key] = state
	return d
}
