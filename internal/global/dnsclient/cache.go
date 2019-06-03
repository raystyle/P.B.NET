package dnsclient

import (
	"errors"
	"sync"
	"time"

	"project/internal/dns"
)

var (
	ERR_INVALID_DEADLINE = errors.New("invalid deadline < 60s or > 1h")
)

type cache struct {
	ipv4_list   []string
	ipv6_list   []string
	update_time time.Time
	rwmutex     sync.RWMutex
}

func (this *DNS) Set_Cache_Deadline(deadline time.Duration) error {
	if deadline < time.Minute || deadline > time.Hour {
		return ERR_INVALID_DEADLINE
	}
	this.caches_rwmutex.Lock()
	this.deadline = deadline
	this.caches_rwmutex.Unlock()
	return nil
}

func (this *DNS) Flush_Cache() {
	this.caches_rwmutex.Lock()
	this.caches = make(map[string]*cache)
	this.caches_rwmutex.Unlock()
}

func (this *DNS) query_cache(domain string, Type dns.Type) []string {
	//clean expire cache
	this.caches_rwmutex.Lock()
	for domain, cache := range this.caches {
		if time.Now().Sub(cache.update_time) > this.deadline {
			delete(this.caches, domain)
		}
	}
	//try query
	if c, exist := this.caches[domain]; exist {
		this.caches_rwmutex.Unlock()
		switch Type {
		case 0, dns.IPV4:
			c.rwmutex.RLock()
			l := len(c.ipv4_list)
			if l != 0 {
				ip_list := make([]string, l)
				copy(ip_list, c.ipv4_list)
				c.rwmutex.RUnlock()
				return ip_list
			} else {
				c.rwmutex.RUnlock()
			}
		case dns.IPV6:
			c.rwmutex.RLock()
			l := len(c.ipv6_list)
			if l != 0 {
				ip_list := make([]string, len(c.ipv6_list))
				copy(ip_list, c.ipv6_list)
				c.rwmutex.RUnlock()
				return ip_list
			} else {
				c.rwmutex.RUnlock()
			}
		}
		return nil
	} else {
		this.caches[domain] = &cache{update_time: time.Now()}
		this.caches_rwmutex.Unlock()
		return nil
	}
}

func (this *DNS) update_cache(domain string, ipv4, ipv6 []string) {
	this.caches_rwmutex.RLock()
	if c, exist := this.caches[domain]; exist {
		this.caches_rwmutex.RUnlock()
		c.rwmutex.Lock()
		if ipv4 != nil {
			c.ipv4_list = ipv4
		}
		if ipv6 != nil {
			c.ipv6_list = ipv6
		}
		c.update_time = time.Now()
		c.rwmutex.Unlock()
	} else {
		this.caches_rwmutex.RUnlock()
	}
}
