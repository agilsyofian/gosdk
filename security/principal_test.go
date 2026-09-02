package security

import "testing"

func TestPrincipalClaim(t *testing.T) {
	principal := &Principal{Claims: map[string]any{"tenant_id": "tenant-1"}}

	value, ok := principal.Claim("tenant_id")
	if !ok || value != "tenant-1" {
		t.Fatalf("Claim(tenant_id) = (%v, %t), want (tenant-1, true)", value, ok)
	}

	if value, ok := principal.Claim("missing"); ok || value != nil {
		t.Fatalf("Claim(missing) = (%v, %t), want (nil, false)", value, ok)
	}

	var nilPrincipal *Principal
	if value, ok := nilPrincipal.Claim("tenant_id"); ok || value != nil {
		t.Fatalf("nil Principal Claim() = (%v, %t), want (nil, false)", value, ok)
	}
}
