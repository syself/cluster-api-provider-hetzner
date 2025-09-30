package models

const (
	// ResetTypePower: Press power button of server. If the server is powered down, it can be turned back on with this function. If the server is still running, it will receive an ACPI signal to shut down. Our servers and standard images are configured so that this process triggers a regular operating system shutdown. What happens is actually exactly the same as what happens when you press the power button on your home computer.
	// Supported by some servers
	ResetTypePower = "power"

	// ResetTypePowerLong: Long power button press. This option forces the server to immediately shut off. It should only be used in cases where the system is unresponsive to a graceful shut-down.
	// Supported by some servers
	ResetTypePowerLong = "power_long"

	// RebootTypeSoftware: Send CTRL+ALT+DEL to the server.
	// Supported by almost all servers.
	RebootTypeSoftware = "sw"

	// ResetTypeHardware: Execute an automatic hardware reset. What happens in the background here is exactly the same as when you press the reset button on your home PC.
	// Supported by all servers.
	ResetTypeHardware = "hw"

	// ResetTypeManual: Order a manual power cycle. The manual power cycle (cold reset) option will generate an email that will be sent directly to our data center. Our technicians will then disconnect your server from the power supply, reconnect it, and thereby restart the system.
	// Supported by all servers. But no recommended for usage via API :-)
	ResetTypeManual = "man"
)

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
