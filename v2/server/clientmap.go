package main

import (
	"sync"
)

//ClientMap connection & domain map
type ClientMap struct {
	locker  sync.Mutex
	clients map[string]*Client
}

func (c *ClientMap) lock() {
	c.locker.Lock()
}

func (c *ClientMap) unlock() {
	c.locker.Unlock()
}

//Append a Client to client map
func (c *ClientMap) Append(client *Client) {
	defer c.unlock()
	c.lock()
	c.clients[client.Domain] = client
}

//Remove client by domain
func (c *ClientMap) Remove(domain string) {
	defer c.unlock()
	c.lock()
	delete(c.clients, domain)
}

//RemoveAll clients
func (c *ClientMap) RemoveAll(domains []string) {
	defer c.unlock()
	c.lock()
	for _, domain := range domains {
		delete(c.clients, domain)
	}
}
