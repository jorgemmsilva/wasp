all: ISCP.bin-runtime

# TODO: solc --pretty-json does not work as advertised?

# also produces ISCP.abi
ISCP.bin-runtime: iscp.sol
	solc --abi --bin-runtime --overwrite iscp.sol -o .

# call `make storage-layout | jq .` for pretty printed json
storage-layout:
	@solc --storage-layout iscp.sol | tail -n +4

.PHONY: storage-layout
