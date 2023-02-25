package models

type CancellationResponse struct {
	Cancellation Cancellation `json:"cancellation"`
}
type Cancellation struct {
	ServerIP                 string      `json:"server_ip"`
	ServerIPv6Net            string      `json:"server_ipv6_net"`
	ServerNumber             int         `json:"server_number"`
	Name                     string      `json:"server_name"`
	EarliestCancellationDate string      `json:"earliest_cancellation_date"`
	Cancelled                bool        `json:"cancelled"`
	ReservationPossible      bool        `json:"reservation_possible"`
	Reservation              bool        `json:"reservation"`
	CancellationDate         string      `json:"cancellation_date"`
	CancellationReason       interface{} `json:"cancellation_reason"`
}
