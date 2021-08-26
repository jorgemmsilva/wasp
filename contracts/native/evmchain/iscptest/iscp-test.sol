// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

pragma solidity >=0.8.0;

import "@iscpcontract/iscp.sol";

contract ISCPTest {
    function getChainId() public view returns (ISCPAddress memory) {
		ISCPAddress memory r = ISCP(ISCP_CONTRACT_ADDRESS).getChainId();
		return r;
    }
}
