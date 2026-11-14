# Everything lives in backend/; these targets run there.
.PHONY: help db db-stop run dev build migrate seed test test-integration vet sqlc docker-build clean

help db db-stop run dev build migrate seed test test-integration vet sqlc docker-build clean:
	@$(MAKE) --no-print-directory -C backend $@
