package dns

import (
	"sync"
	"time"
)

type cache struct {
	ipv4List   []string
	ipv6List   []string
	updateTime time.Time
	rwm        sync.RWMutex
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
		var ip []string
		switch typ {
		case TypeIPv4:
			cache.rwm.RLock()
			defer cache.rwm.RUnlock()
			ip = cache.ipv4List
		case TypeIPv6:
			cache.rwm.RLock()
			defer cache.rwm.RUnlock()
			ip = cache.ipv6List
		}
		// must copy
		cp := make([]string, len(ip))
		copy(cp, ip)
		return cp
	}
	// create cache object
	c.caches[domain] = &cache{updateTime: time.Now()}
	return nil
}

func (c *Client) updateCache(domain, typ string, ip []string) {
	// must copy
	cp := make([]string, len(ip))
	copy(cp, ip)

	c.cachesRWM.RLock()
	defer c.cachesRWM.RUnlock()
	if cache, ok := c.caches[domain]; ok {
		cache.rwm.Lock()
		defer cache.rwm.Unlock()
		switch typ {
		case TypeIPv4:
			cache.ipv4List = cp
		case TypeIPv6:
			cache.ipv6List = cp
		}
		cache.updateTime = time.Now()
	}
}
