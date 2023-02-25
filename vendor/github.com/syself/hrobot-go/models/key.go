package models

type KeyResponse struct {
	Key Key `json:"key"`
}

type KeySetInput struct {
	Name string `json:"name"`
	Data string `json:"data"`
}

type Key struct {
	Name        string `json:"name"`
	Fingerprint string `json:"fingerprint"`
	Type        string `json:"type"`
	Size        int    `json:"size"`
	Data        string `json:"data"`
}
