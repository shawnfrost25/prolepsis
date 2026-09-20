REDIS_PASS ?= placeholder
REDIS_PATH ?= placeholder

.PHONY: redis-test

redis-test:
	redis-cli -a ${REDIS_PASS} < ${REDIS_PATH}
