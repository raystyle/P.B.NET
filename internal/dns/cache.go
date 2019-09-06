package dns

import (
	"errors"
	"sync"
	"time"
)

const (
	defaultCacheDeadline = 1 * time.Minute
)

var (
	ErrInvalidDeadline = errors.New("deadline < 60s or > 1h")
)

type cache struct {
	ipv4List   []string
	ipv6List   []string
	updateTime time.Time
	rwm        sync.RWMutex
}

func (c *Client) SetCacheDeadline(deadline time.Duration) error {
	if deadline < time.Minute || deadline > time.Hour {
		return ErrInvalidDeadline
	}
	c.cachesRWM.Lock()
	c.deadline = deadline
	c.cachesRWM.Unlock()
	return nil
}

func (c *Client) GetCacheDeadline() time.Duration {
	c.cachesRWM.RLock()
	deadline := c.deadline
	c.cachesRWM.RUnlock()
	return deadline
}

func (c *Client) FlushCache() {
	c.cachesRWM.Lock()
	c.caches = make(map[string]*cache)
	c.cachesRWM.Unlock()
}

func (c *Client) queryCache(domain string, Type Type) []string {
	// clean expire cache
	c.cachesRWM.Lock()
	for domain, cache := range c.caches {
		if time.Now().Sub(cache.updateTime) > c.deadline {
			delete(c.caches, domain)
		}
	}
	// try query
	if _cache, ok := c.caches[domain]; ok {
		c.cachesRWM.Unlock()
		switch Type {
		case IPv4:
			_cache.rwm.RLock()
			l := len(_cache.ipv4List)
			if l != 0 {
				ipList := make([]string, l)
				copy(ipList, _cache.ipv4List)
				_cache.rwm.RUnlock()
				return ipList
			} else {
				_cache.rwm.RUnlock()
			}
		case IPv6:
			_cache.rwm.RLock()
			l := len(_cache.ipv6List)
			if l != 0 {
				ipList := make([]string, len(_cache.ipv6List))
				copy(ipList, _cache.ipv6List)
				_cache.rwm.RUnlock()
				return ipList
			} else {
				_cache.rwm.RUnlock()
			}
		}
		return nil
	} else {
		c.caches[domain] = &cache{updateTime: time.Now()}
		c.cachesRWM.Unlock()
		return nil
	}
}

func (c *Client) updateCache(domain string, ipv4, ipv6 []string) {
	c.cachesRWM.RLock()
	if cache, ok := c.caches[domain]; ok {
		c.cachesRWM.RUnlock()
		cache.rwm.Lock()
		if ipv4 != nil {
			cache.ipv4List = ipv4
		}
		if ipv6 != nil {
			cache.ipv6List = ipv6
		}
		cache.updateTime = time.Now()
		cache.rwm.Unlock()
	} else {
		c.cachesRWM.RUnlock()
	}
}
