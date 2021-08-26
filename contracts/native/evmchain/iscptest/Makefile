all: ISCPTest.bin

# also produces ISCPTest.abi
ISCPTest.bin: iscp-test.sol
	solc --abi --bin --overwrite @iscpcontract=$(realpath ../iscpcontract) iscp-test.sol -o .
	rm ISCP.*
