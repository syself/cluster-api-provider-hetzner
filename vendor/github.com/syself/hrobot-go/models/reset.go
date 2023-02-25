package models

const ResetTypePower = "power"
const ResetTypeHardware = "hw"
const ResetTypeManual = "man"

type ResetResponse struct {
	Reset Reset `json:"reset"`
}

type Reset struct {
	ServerIP        string   `json:"server_ip"`
	ServerIPv6Net   string   `json:"server_ipv6_net"`
	ServerNumber    int      `json:"server_number"`
	Type            []string `json:"type"`
	OperatingStatus string   `json:"operating_status"`
}

type ResetPostResponse struct {
	Reset ResetPost `json:"reset"`
}

type ResetPost struct {
	ServerIP      string `json:"server_ip"`
	ServerIPv6Net string `json:"server_ipv6_net"`
	ServerNumber  int    `json:"server_number"`
	Type          string `json:"type"`
}

type ResetSetInput struct {
	Type string
}
