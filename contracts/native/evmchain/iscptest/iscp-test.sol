// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

pragma solidity >=0.8.0;

contract ISCPTest {
    address constant public iscpAddress = 0x0000000000000000000000000000000000001074;

    function getChainId() public view returns (bytes32) {
        return ISCP(iscpAddress).chainId();
    }
}

interface ISCP {
    function chainId() external view returns (bytes32);
}
