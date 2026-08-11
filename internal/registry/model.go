package registry

import "time"

type Status string

const (
	StatusActive  Status = "active"
	StatusDeleted Status = "deleted"
)

type ServiceRecord struct {
	Name       string    `json:"name"`
	Status     Status    `json:"status"`
	Version    uint64    `json:"version"`
	OriginNode string    `json:"originNode"`
	UpdatedAt  time.Time `json:"updatedAt"`
}

type InstanceRecord struct {
	ID          string    `json:"id"`
	ServiceName string    `json:"serviceName"`
	Address     string    `json:"address"`
	Port        int       `json:"port"`
	Status      Status    `json:"status"`
	Version     uint64    `json:"version"`
	OriginNode  string    `json:"originNode"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type InstanceInput struct {
	Address string `json:"address"`
	Port    int    `json:"port"`
}
