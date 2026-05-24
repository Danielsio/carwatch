package main

import "testing"

func TestDefaultHealthBind(t *testing.T) {
	if defaultHealthBind != "0.0.0.0:8083" {
		t.Errorf("defaultHealthBind = %q, want 0.0.0.0:8083", defaultHealthBind)
	}
}
