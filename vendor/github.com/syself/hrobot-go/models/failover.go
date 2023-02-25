package models

type FailoverResponse struct {
	Failover Failover `json:"failover"`
}

type Failover struct {
	IP             string `json:"ip"`
	Netmask        string `json:"netmask"`
	ServerIP       string `json:"server_ip"`
	ServerNumber   int    `json:"server_number"`
	ActiveServerIP string `json:"active_server_ip"`
}
