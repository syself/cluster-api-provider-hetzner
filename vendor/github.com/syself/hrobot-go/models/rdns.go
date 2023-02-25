package models

type RdnsResponse struct {
	Rdns Rdns `json:"rdns"`
}

type Rdns struct {
	IP  string `json:"ip"`
	Ptr string `json:"ptr"`
}
