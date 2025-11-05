package main

import (
	rfsm "github.com/ethan/rfsm/src"
)

func DefineFiatCryptoFlow() *rfsm.Definition {
	// Composite groups: FIAT / HEDGE / CRYPTO with single internal state
	fiatSub, _ := rfsm.NewDef("FIAT_SUB").
		State("fiat_internal", rfsm.WithInitial(), rfsm.WithFinal()).
		Current("fiat_internal").
		Build()

	hedgeSub, _ := rfsm.NewDef("HEDGE_SUB").
		State("hedge_internal", rfsm.WithInitial(), rfsm.WithFinal()).
		Current("hedge_internal").
		Build()

	cryptoSub, _ := rfsm.NewDef("CRYPTO_SUB").
		State("crypto_internal", rfsm.WithInitial(), rfsm.WithFinal()).
		Current("crypto_internal").
		Build()

	def, _ := rfsm.NewDef("FiatCryptoFlow").
		// Terminal states
		State("SUCCESS", rfsm.WithFinal()).
		State("FAILED", rfsm.WithFinal()).
		State("REFUNDED", rfsm.WithFinal()).
		State("EXPIRED", rfsm.WithFinal()).
		// Groups
		State("FIAT", rfsm.WithSubDef(fiatSub)).
		State("HEDGE", rfsm.WithSubDef(hedgeSub)).
		State("CRYPTO", rfsm.WithSubDef(cryptoSub)).
		// Other states
		State("INIT", rfsm.WithInitial()).
		State("PENDING_FIAT_DEPOSIT").
		State("PENDING_FIAT_EXPIRED").
		State("PENDING_FIAT_REFUND").
		State("PENDING_FIAT_DEPOSITED").
		State("PENDING_FIAT_DEPOSIT_FAILED").
		State("PENDING_HEDGE_REQUOTE").
		State("PENDING_HEDGE_EXECUTED").
		State("PENDING_HEDGE_FAILED").
		State("PENDING_HEDGE_UNWIND").
		State("PENDING_CRYPTO_WITHDRAW").
		State("PENDING_CRYPTO_WITHDRAW_FAILED").
		State("PENDING_CRYPTO_WITHDRAWN").
		Current("INIT").

		// ---- FIAT Stage ----
		On(rfsm.TransitionKey{From: "PENDING_FIAT_DEPOSIT", To: "FIAT"}, rfsm.WithName("start_fiat")).
		On(rfsm.TransitionKey{From: "FIAT", To: "PENDING_FIAT_EXPIRED"}, rfsm.WithName("overdue")).
		On(rfsm.TransitionKey{From: "FIAT", To: "PENDING_FIAT_REFUND"}, rfsm.WithName("manual_refund")).
		On(rfsm.TransitionKey{From: "FIAT", To: "PENDING_FIAT_DEPOSITED"}, rfsm.WithName("success")).
		On(rfsm.TransitionKey{From: "FIAT", To: "PENDING_FIAT_DEPOSIT_FAILED"}, rfsm.WithName("failed")).
		On(rfsm.TransitionKey{From: "PENDING_FIAT_REFUND", To: "REFUNDED"}, rfsm.WithName("refund")).
		On(rfsm.TransitionKey{From: "PENDING_FIAT_DEPOSIT_FAILED", To: "FAILED"}, rfsm.WithName("to_failed")).
		On(rfsm.TransitionKey{From: "PENDING_FIAT_EXPIRED", To: "EXPIRED"}, rfsm.WithName("expire")).

		// ---- HEDGE Stage ----
		On(rfsm.TransitionKey{From: "PENDING_FIAT_DEPOSITED", To: "HEDGE"}, rfsm.WithName("to_hedge")).
		On(rfsm.TransitionKey{From: "PENDING_HEDGE_REQUOTE", To: "HEDGE"}, rfsm.WithName("requote")).
		On(rfsm.TransitionKey{From: "HEDGE", To: "PENDING_HEDGE_EXECUTED"}, rfsm.WithName("executed")).
		On(rfsm.TransitionKey{From: "HEDGE", To: "PENDING_HEDGE_FAILED"}, rfsm.WithName("failed")).
		On(rfsm.TransitionKey{From: "PENDING_HEDGE_FAILED", To: "PENDING_HEDGE_REQUOTE"}, rfsm.WithName("retry_hedge")).
		// Optional unwind paths
		On(rfsm.TransitionKey{From: "PENDING_HEDGE_EXECUTED", To: "PENDING_HEDGE_UNWIND"}, rfsm.WithName("revert_unwind")).
		On(rfsm.TransitionKey{From: "PENDING_HEDGE_UNWIND", To: "PENDING_HEDGE_REQUOTE"}, rfsm.WithName("requote")).
		On(rfsm.TransitionKey{From: "PENDING_HEDGE_UNWIND", To: "PENDING_FIAT_REFUND"}, rfsm.WithName("cancel_trade")).

		// Proceed to crypto
		On(rfsm.TransitionKey{From: "PENDING_HEDGE_EXECUTED", To: "PENDING_CRYPTO_WITHDRAW"}, rfsm.WithName("to_crypto")).

		// ---- CRYPTO Stage ----
		On(rfsm.TransitionKey{From: "PENDING_CRYPTO_WITHDRAW", To: "CRYPTO"}, rfsm.WithName("start_crypto")).
		On(rfsm.TransitionKey{From: "CRYPTO", To: "PENDING_CRYPTO_WITHDRAW_FAILED"}, rfsm.WithName("failed")).
		On(rfsm.TransitionKey{From: "CRYPTO", To: "PENDING_CRYPTO_WITHDRAWN"}, rfsm.WithName("success")).
		On(rfsm.TransitionKey{From: "PENDING_CRYPTO_WITHDRAW_FAILED", To: "CRYPTO"}, rfsm.WithName("retry")).
		On(rfsm.TransitionKey{From: "PENDING_CRYPTO_WITHDRAW_FAILED", To: "PENDING_HEDGE_UNWIND"}, rfsm.WithName("unwind")).
		On(rfsm.TransitionKey{From: "PENDING_CRYPTO_WITHDRAWN", To: "SUCCESS"}, rfsm.WithName("to_success")).

		// ---- Initial fan-in/out ----
		On(rfsm.TransitionKey{From: "INIT", To: "PENDING_FIAT_DEPOSIT"}, rfsm.WithName("init_next")).
		Build()

	return def
}
