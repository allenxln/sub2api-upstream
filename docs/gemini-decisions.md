# Gemini Decisions API

Gemini API-key accounts can serve Google's native, non-streaming Decisions API
through `POST /v1beta/decisions`. The gateway uses its existing key/group
permissions, account scheduling, configured account proxy and usage accounting.
The account must declare the requested model in its native model mapping.

```sh
curl "$SUB2API_BASE_URL/v1beta/decisions" \
  -H "x-goog-api-key: $SUB2API_KEY" \
  -H 'Content-Type: application/json' \
  -d '{
    "model": "gemini-decision-1",
    "input": "The blue mug arrived cracked.",
    "questions": {
      "damaged": {
        "type": "noul",
        "instructions": "Did the item arrive damaged?"
      }
    }
  }'
```

`Authorization: Bearer` with a gateway key also works. Upstream credentials stay
on the account; the gateway sends them only in Google's `x-goog-api-key` header.
Native aliases rewrite the model without changing shared text/media input,
question criteria, answer probabilities or provider extensions. Decisions does
not use `generateContent`, streaming, thinking tokens or context caching.

The provider's `usage.total_input_tokens` is the billing input; output usage is
zero. The paid reference fallback is USD 0.10 per million input tokens. Channel
price cards and group/user multipliers follow normal native configuration.
An upstream free tier does not override the gateway's configured customer price.

Bigpot's production composite group exposes `gemini-decision-1` at
`https://coding-api.guazi-corp.com/v1beta/decisions`. Existing group API keys work
without rotation. The CNY channel price is 0.65 per million input tokens and
zero output; the default 0.3 group factor gives CNY 0.195/M input. Individual
overrides continue to follow native billing rules. The endpoint requires the
Decisions request shape above; a Jev or chat-generation payload is not compatible.
The deployed account reuses the existing `bestapi-egress` proxy, verified by a
real request from the production container. Actual project quota was not readable
with the current console permissions, so published free-tier limits are not a
verified account entitlement.

For a Decisions-only account, the administrator connection test chooses its
declared model and shows the native decision JSON. It does not send a chat
generation request. Production proxy reachability and actual project quota must
be verified from the deployment's egress, rather than inferred from a developer's
browser or subscription.

Google's free tier may use inputs and outputs to improve its products. Consult
the current terms and avoid submitting sensitive or personal information on
that tier.

Official references: [protocol](https://aistudio.google.com/docs/decisions-api),
[pricing](https://aistudio.google.com/docs/pricing#gemini-decision),
[quotas](https://aistudio.google.com/docs/rate-limits#gemini-decision).
