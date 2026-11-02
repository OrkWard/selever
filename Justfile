default: test

build:
    nimble build -d:release -y

test:
    nimble test -y

benchmark direct shim iterations="200":
    nim c -d:release --hints:off --out:bin/shim-latency bench/shim_latency.nim
    bin/shim-latency {{ direct }} {{ shim }} {{ iterations }} --version
