package controllers

import (
	"context"
	"errors"
	"testing"

	"github.com/shopspring/decimal"
	"github.com/tertua/tupay/platform/gateway"
)

type checkableGateway struct {
	fakeGateway
	min       decimal.Decimal
	minErr    error
	seenAsset *string
}

func (g checkableGateway) Methods() []string { return []string{gateway.MethodCrypto} }
func (g checkableGateway) MinAmount(_ context.Context, _, payCurrency string) (decimal.Decimal, error) {
	if g.seenAsset != nil {
		*g.seenAsset = payCurrency
	}
	return g.min, g.minErr
}

func TestLiveMinimum(t *testing.T) {
	ctx := context.Background()
	spec := chargeSpec{AmountDecimal: "3.08", Currency: "USD"}
	// The minimum asked for belongs to the asset the payer picked, never to a
	// hard-coded default.
	var asked string
	below := checkableGateway{fakeGateway: fakeGateway{name: "nowpayments"}, min: decimal.RequireFromString("18.81"), seenAsset: &asked}
	limit, isBelow := liveMinimum(ctx, below, gateway.MethodCrypto, spec, "usdttrc20")
	if !isBelow {
		t.Fatal("3.08 under an 18.81 minimum must be reported as below")
	}
	if !limit.Equal(decimal.RequireFromString("18.81")) {
		t.Fatalf("reported minimum %s, want 18.81 so the payer can be told", limit)
	}
	if asked != "usdttrc20" {
		t.Fatalf("minimum asked for asset %q, want usdttrc20", asked)
	}
	above := checkableGateway{fakeGateway: fakeGateway{name: "nowpayments"}, min: decimal.RequireFromString("1.00")}
	if _, isBelow := liveMinimum(ctx, above, gateway.MethodCrypto, spec, "ltc"); isBelow {
		t.Fatal("3.08 over a 1.00 minimum must stay usable")
	}
	if _, isBelow := liveMinimum(ctx, below, gateway.MethodBankTransfer, spec, "usdttrc20"); isBelow {
		t.Fatal("fiat methods have no provider minimum")
	}
	// No asset picked yet: no asset's minimum applies, so crypto is offered
	// and gated only once the payer chooses one.
	asked = ""
	unpicked := checkableGateway{fakeGateway: fakeGateway{name: "nowpayments"}, min: decimal.RequireFromString("18.81"), seenAsset: &asked}
	if _, isBelow := liveMinimum(ctx, unpicked, gateway.MethodCrypto, spec, ""); isBelow {
		t.Fatal("an unpicked asset must not mark the method list")
	}
	if asked != "" {
		t.Fatalf("no asset chosen, but a minimum was fetched for %q", asked)
	}
	broken := checkableGateway{fakeGateway: fakeGateway{name: "nowpayments"}, minErr: errors.New("boom")}
	if _, isBelow := liveMinimum(ctx, broken, gateway.MethodCrypto, spec, "usdttrc20"); isBelow {
		t.Fatal("a minimum-fetch failure must fail open (keep the method usable)")
	}
	if _, isBelow := liveMinimum(ctx, fakeGateway{name: "midtrans"}, gateway.MethodCrypto, spec, "usdttrc20"); isBelow {
		t.Fatal("providers without a checker must never mark a method")
	}
}
