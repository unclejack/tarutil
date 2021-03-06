all: test

install_box:
	@sh install_box.sh

install_box_ci:
	@sh install_box_ci.sh

build: 
	PATH=${PATH}:${PWD}/bin box -t box-builder/tarutil build.rb	

checks:
	@docker run --rm box-builder/tarutil bash checks.sh

run_test: checks
	docker run --rm box-builder/tarutil

test: install_box build run_test

test-ci: install_box_ci build run_test

.PHONY: build install_box
