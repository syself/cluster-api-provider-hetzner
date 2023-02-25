package models

type ServerResponse struct {
	Server Server `json:"server"`
}

type Subnet struct {
	IP              string `json:"ip"`
	Mask            string `json:"mask"`
	Gateway         string `json:"gateway"`
	ServerIP        string `json:"server_ip"`
	ServerNumber    int    `json:"server_number"`
	Failover        bool   `json:"failover"`
	Locked          bool   `json:"locked"`
	TrafficWarnings bool   `json:"traffic_warnings"`
	TrafficHourly   int    `json:"traffic_hourly"`
	TrafficDaily    int    `json:"traffic_daily"`
	TrafficMonthly  int    `json:"traffic_monthly"`
}

type Server struct {
	ServerIP         string   `json:"server_ip"`
	ServerIPv6Net    string   `json:"server_ipv6_net"`
	ServerNumber     int      `json:"server_number"`
	Name             string   `json:"server_name"`
	Product          string   `json:"product"`
	Dc               string   `json:"dc"`
	Traffic          string   `json:"traffic"`
	Status           string   `json:"status"`
	Cancelled        bool     `json:"cancelled"`
	PaidUntil        string   `json:"paid_until"`
	IP               []string `json:"ip"`
	Subnet           []Subnet `json:"subnet"`
	Reset            bool     `json:"reset"`
	Rescue           bool     `json:"rescue"`
	Vnc              bool     `json:"vnc"`
	Windows          bool     `json:"windows"`
	Plesk            bool     `json:"plesk"`
	Cpanel           bool     `json:"cpanel"`
	Wol              bool     `json:"wol"`
	HotSwap          bool     `json:"hot_swap"`
	LinkedStoragebox int      `json:"linked_storagebox"`
}

type ServerSetNameInput struct {
	Name string
}
