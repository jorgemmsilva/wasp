// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

pragma solidity >=0.8.0;

import "@iscpcontract/iscp.sol";

ISCP constant iscp = ISCP(ISCP_CONTRACT_ADDRESS);

contract ISCPTest {
	bytes32 _entropy;

    function getChainId() public view returns (ISCPAddress memory) {
		ISCPAddress memory r = iscp.getChainId();
		return r;
    }

	function storeEntropy() public {
		_entropy = iscp.getEntropy();
	}

	function getEntropy() public view returns (bytes32) {
		return _entropy;
	}
}
