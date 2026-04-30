package billing

func recordBilling(instanceId string, increment func(*billingInfo)) {
	billing.mu.RLock()
	info, ok := billing.internalMap[instanceId]
	if ok {
		increment(info)
		billing.mu.RUnlock()
		return
	}
	billing.mu.RUnlock()

	billing.mu.Lock()
	if info, ok = billing.internalMap[instanceId]; ok {
		increment(info)
		billing.mu.Unlock()
		return
	}
	info = &billingInfo{}
	billing.internalMap[instanceId] = info
	billing.mu.Unlock()
	increment(info)
}

func RedisRead(instanceId string) {
	recordBilling(instanceId, func(b *billingInfo) { b.redisReads.Add(1) })
}

func RedisWrite(instanceId string) {
	recordBilling(instanceId, func(b *billingInfo) { b.redisWrites.Add(1) })
}

func RedisPublish(instanceId string) {
	recordBilling(instanceId, func(b *billingInfo) { b.redisPublishes.Add(1) })
}

func FirestoreWrite(instanceId string, bytes uint64) {
	recordBilling(instanceId, func(b *billingInfo) {
		b.firestoreWrites.Add(1)
		b.firestoreWriteBytes.Add(bytes)
	})
}

func FirestoreRead(instanceId string, bytes uint64) {
	recordBilling(instanceId, func(b *billingInfo) {
		b.firestoreReads.Add(1)
		b.firestoreReadBytes.Add(bytes)
	})
}

func OutboundFetch(instanceId string, bytes uint64) {
	recordBilling(instanceId, func(b *billingInfo) {
		b.outboundFetchs.Add(1)
		b.outboundFetchBytes.Add(bytes)
	})
}

func InboundRequest(instanceId string) {
	recordBilling(instanceId, func(b *billingInfo) { b.inboundRequests.Add(1) })
}

func OutboundBytes(instanceId string, bytes uint64) {
	recordBilling(instanceId, func(b *billingInfo) { b.outboundBytes.Add(bytes) })
}
