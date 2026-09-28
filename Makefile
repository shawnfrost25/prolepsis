REDIS_PASS ?= placeholder
REDIS_PATH ?= placeholder

.PHONY: redis-test

redis-test:
	redis-cli -a ${REDIS_PASS} < ${REDIS_PATH}

manual-test:
	redis-cli -a ${REDIS_PASS} < SET "session:token:bd646ee75b123d28cf646551a035fc85b8d543742d92a2a489ba386a67ba4490" ""
