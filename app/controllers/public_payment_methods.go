package controllers

import (
	"context"
	"strings"

	"github.com/shopspring/decimal"
	"github.com/tertua/tupay/app/models"
	"github.com/tertua/tupay/platform/gateway"
)

// publicChargeMethod is one selectable method on the public pay page. The
// amount is what the provider would actually charge (already converted from
// the invoice currency with the owner's manual rate).
type publicChargeMethod struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Currency     string `json:"currency"`
	Amount       string `json:"amount"`
	BelowMinimum bool   `json:"below_minimum,omitempty"`
	MinAmount    string `json:"min_amount,omitempty"`
}

// availableChargeMethods lists provider-neutral methods that can settle the
// invoice balance, one entry per method, sorted by id. Providers whose
// currency cannot be reached with the configured rate are skipped, so the
// payer never sees a method that would fail at intent creation. The owner's
// per-provider allowlist (gateway_allowlist.go) narrows the default provider's
// methods; a nil allowlist allows them all.
// Providers reporting a live minimum (gateway.MinAmountChecker, cached
// hourly) do not drop a method whose charge sits below it: the row stays in
// the list marked below_minimum with the provider's own min_amount, so the
// payer is told why it cannot be used instead of watching it vanish. The
// minimum is per asset, so an asset-agnostic list (payCurrency empty) offers
// crypto unmarked and lets the asset's own minimum mark it once picked. Any
// minimum-fetch failure keeps the method usable (fail-open) and lets intent
// creation return the authoritative error.
// Later: group and order this list per audience (IDR methods first for domestic payers) while keeping publicChargeMethod unchanged.
func availableChargeMethods(ctx context.Context, invoiceCurrency string, balance, usdToIdr decimal.Decimal, settings models.Settings, payCurrency string) []publicChargeMethod {
	out := make([]publicChargeMethod, 0)
	for _, ref := range gateway.OfferedMethods() {
		if !methodAllowedFor(ref.Provider.Name(), providerAllowlist(settings, ref.Provider.Name()), ref.ID) {
			continue
		}
		spec, err := buildCharge(ref.Provider, invoiceCurrency, balance, usdToIdr)
		if err != nil {
			continue
		}
		entry := publicChargeMethod{
			ID:       ref.ID,
			Name:     gateway.MethodName(ref.ID),
			Currency: spec.Currency,
			Amount:   spec.chargeAmount(),
		}
		if limit, below := liveMinimum(ctx, ref.Provider, ref.ID, spec, payCurrency); below {
			entry.BelowMinimum = true
			entry.MinAmount = limit.String()
		}
		out = append(out, entry)
	}
	return out
}

// liveMinimum returns the provider's live minimum for a charge and whether
// the charge sits below it. Only crypto is checked (fiat methods have no
// provider minimum) and an empty payCurrency (no asset picked yet) checks
// nothing, because no asset's minimum applies. A fetch error reports no
// limit, so a provider hiccup never blocks a payment intent creation may
// still accept.
func liveMinimum(ctx context.Context, gw gateway.Gateway, method string, spec chargeSpec, payCurrency string) (decimal.Decimal, bool) {
	if method != gateway.MethodCrypto || strings.TrimSpace(payCurrency) == "" {
		return decimal.Zero, false
	}
	checker, ok := gw.(gateway.MinAmountChecker)
	if !ok {
		return decimal.Zero, false
	}
	charge, err := decimal.NewFromString(spec.AmountDecimal)
	if err != nil {
		return decimal.Zero, false
	}
	limit, err := checker.MinAmount(ctx, spec.Currency, payCurrency)
	if err != nil {
		return decimal.Zero, false
	}
	return limit, charge.LessThan(limit)
}
