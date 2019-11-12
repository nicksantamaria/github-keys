package main

// Key stores a ssh public key and basic comment.
type Key struct {
	Comment string `json:"comment"`
	Key     string `json:"key"`
}
