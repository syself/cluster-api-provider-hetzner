package models

type IPResponse struct {
	IP IP `json:"ip"`
}

type IP struct {
	IP              string `json:"ip"`
	Gateway         string `json:"gateway"`
	Mask            int    `json:"mask"`
	Broadcast       string `json:"broadcast"`
	ServerIP        string `json:"server_ip"`
	ServerNumber    int    `json:"server_number"`
	Locked          bool   `json:"locked"`
	SeparateMac     string `json:"separate_mac"`
	TrafficWarnings bool   `json:"traffic_warnings"`
	TrafficHourly   int    `json:"traffic_hourly"`
	TrafficDaily    int    `json:"traffic_daily"`
	TrafficMonthly  int    `json:"traffic_monthly"`
}
