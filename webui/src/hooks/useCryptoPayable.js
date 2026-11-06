import { useEffect, useState } from "react";
import { publicPayApi } from "@/api/publicPay";

// Preflight the crypto minimum for the asset the payer picked. The page's
// own method list is fetched without a pay currency (no asset chosen yet, so
// crypto is never hidden by some default asset's minimum); once an asset is
// picked this re-reads it with ?pay_currency= and the backend applies that
// asset's own live minimum. Fail-open — on any fetch error intent creation
// stays the authoritative check.
// Seam: (token, asset, active) — `active` only runs the request while the
// payer is still choosing, never while a payment is being created.
export function useCryptoPayable(token, asset, active) {
  const [state, setState] = useState({ checking: false, payable: true, minAmount: "", minCurrency: "" });

  useEffect(() => {
    if (!active || !asset) {
      setState({ checking: false, payable: true, minAmount: "", minCurrency: "" });
      return undefined;
    }
    let cancelled = false;
    setState({ checking: true, payable: true, minAmount: "", minCurrency: "" });
    publicPayApi
      .get(token, asset)
      .then((res) => {
        if (cancelled) return;
        const crypto = (res?.methods || []).find((m) => m.id === "crypto");
        const blocked = !!crypto?.below_minimum;
        setState({
          checking: false,
          payable: !blocked,
          minAmount: blocked ? crypto.min_amount : "",
          minCurrency: crypto?.currency || "",
        });
      })
      .catch(() => {
        if (!cancelled) setState({ checking: false, payable: true, minAmount: "", minCurrency: "" });
      });
    return () => {
      cancelled = true;
    };
  }, [token, asset, active]);

  return state;
}
