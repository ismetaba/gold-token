package channels

// InApp delivery is handled directly by the consumer inserting a delivery
// record into the database. No external transport is required — the record
// itself is the notification surface for the API.
//
// This file documents the contract: in-app notifications are marked "sent"
// immediately upon DB insertion and surfaced via GET /notifications.
