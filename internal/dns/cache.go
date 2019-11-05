package dns

import (
	"errors"
	"sync"
	"time"
)

var (
	ErrInvalidExpireTime = errors.New("expire time < 60 second or > 1 hour")
)

type cache struct {
	ipv4List   []string
	ipv6List   []string
	updateTime time.Time
	rwm        sync.RWMutex
}

func (c *Client) GetCacheExpireTime() time.Duration {
	c.cachesRWM.RLock()
	defer c.cachesRWM.RUnlock()
	expire := c.expire
	return expire
}

func (c *Client) SetCacheExpireTime(expire time.Duration) error {
	if expire < time.Minute || expire > time.Hour {
		return ErrInvalidExpireTime
	}
	c.cachesRWM.Lock()
	defer c.cachesRWM.Unlock()
	c.expire = expire
	return nil
}

func (c *Client) FlushCache() {
	c.cachesRWM.Lock()
	defer c.cachesRWM.Unlock()
	c.caches = make(map[string]*cache)
}

func (c *Client) queryCache(domain, typ string) []string {
	// clean expire cache
	c.cachesRWM.Lock()
	defer c.cachesRWM.Unlock()
	for domain, cache := range c.caches {
		if time.Now().Sub(cache.updateTime) > c.expire {
			delete(c.caches, domain)
		}
	}
	// query cache
	if cache, ok := c.caches[domain]; ok {
		var result []string
		switch typ {
		case TypeIPv4:
			cache.rwm.RLock()
			defer cache.rwm.RUnlock()
			result = cache.ipv4List
		case TypeIPv6:
			cache.rwm.RLock()
			defer cache.rwm.RUnlock()
			result = cache.ipv6List
		}
		return result
	}
	// create cache object
	c.caches[domain] = &cache{updateTime: time.Now()}
	return nil
}

func (c *Client) updateCache(domain string, ipv4, ipv6 []string) {
	c.cachesRWM.RLock()
	defer c.cachesRWM.RUnlock()
	if cache, ok := c.caches[domain]; ok {
		cache.rwm.Lock()
		defer cache.rwm.Unlock()
		if len(ipv4) != 0 {
			cache.ipv4List = ipv4
		}
		if len(ipv6) != 0 {
			cache.ipv6List = ipv6
		}
		cache.updateTime = time.Now()
	}
}
