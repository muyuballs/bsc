package main

import (
	"sync"
)

type ClientMap struct {
	locker  sync.Mutex
	clients map[string]*Client
}

func (c *ClientMap) Lock() {
	c.locker.Lock()
}

func (c *ClientMap) Unlock() {
	c.locker.Unlock()
}

func (c *ClientMap) Append(client *Client) {
	defer c.Unlock()
	c.Lock()
	c.clients[client.Domain] = client
}

func (c *ClientMap) Remove(domain string) {
	defer c.Unlock()
	c.Lock()
	delete(c.clients, domain)
}

func (c *ClientMap) RemoveAll(domains []string) {
	defer c.Unlock()
	c.Lock()
	for _, domain := range domains {
		delete(c.clients, domain)
	}
}
