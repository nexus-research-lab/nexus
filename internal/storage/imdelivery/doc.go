// Package imdelivery owns durable IM delivery origins and reply admission.
// L2 | Parent: internal/storage. model.go holds immutable identities;
// repository.go stores scoped intents, send claims and one-shot reply dispatch.
// Inbound human evidence remains in the Channels ingress ledger.
package imdelivery
