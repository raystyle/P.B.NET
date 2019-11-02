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
	expire := c.expire
	c.cachesRWM.RUnlock()
	return expire
}

func (c *Client) SetCacheExpireTime(expire time.Duration) error {
	if expire < time.Minute || expire > time.Hour {
		return ErrInvalidExpireTime
	}
	c.cachesRWM.Lock()
	c.expire = expire
	c.cachesRWM.Unlock()
	return nil
}

func (c *Client) FlushCache() {
	c.cachesRWM.Lock()
	c.caches = make(map[string]*cache)
	c.cachesRWM.Unlock()
}

func (c *Client) queryCache(domain, typ string) []string {
	// clean expire cache
	c.cachesRWM.Lock()
	for domain, cache := range c.caches {
		if time.Now().Sub(cache.updateTime) > c.expire {
			delete(c.caches, domain)
		}
	}
	// query
	if cache, ok := c.caches[domain]; ok {
		c.cachesRWM.Unlock()
		switch typ {
		case TypeIPv4:
			cache.rwm.RLock()
			ipList := cache.ipv4List
			cache.rwm.RUnlock()
			return ipList
		case TypeIPv6:
			cache.rwm.RLock()
			ipList := cache.ipv6List
			cache.rwm.RUnlock()
			return ipList
		default:
			// <security> In theory,
			// it's never going to work here
			return nil
		}
	}
	// create cache object
	c.caches[domain] = &cache{updateTime: time.Now()}
	c.cachesRWM.Unlock()
	return nil
}

func (c *Client) updateCache(domain string, ipv4, ipv6 []string) {
	c.cachesRWM.RLock()
	if cache, ok := c.caches[domain]; ok {
		c.cachesRWM.RUnlock()
		cache.rwm.Lock()
		if len(ipv4) != 0 {
			cache.ipv4List = ipv4
		}
		if len(ipv6) != 0 {
			cache.ipv6List = ipv6
		}
		cache.updateTime = time.Now()
		cache.rwm.Unlock()
	} else {
		// <security> In theory,
		// it's never going to work here
		c.cachesRWM.RUnlock()
	}
}
