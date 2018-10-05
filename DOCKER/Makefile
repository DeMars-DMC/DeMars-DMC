build:
	@sh -c "'$(CURDIR)/build.sh'"

push:
	@sh -c "'$(CURDIR)/push.sh'"

build_develop:
	docker build -t "Demars-DMC/Demars-DMC:develop" -f Dockerfile.develop .

build_testing:
	docker build --tag Demars-DMC/testing -f ./Dockerfile.testing .

push_develop:
	docker push "Demars-DMC/Demars-DMC:develop"

.PHONY: build build_develop push push_develop
