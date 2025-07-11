.PHONY: build run test clean



build:
	go build -o ./build/kpi-metricsd .

run:
	./build/kpi-metricsd

test:
	go test -v ./...

clean:
	rm -rf ./build;
	rm -rf ./testdata