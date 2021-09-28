// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

pragma solidity ^0.8.3;

import "./IOTAToken.sol";

contract IOTAFactory {
    mapping(bytes => address) internal _tokenAddresses;

    // event TokenCreated(address tokenAddress);

    function deployNewToken(bytes memory tokenId) internal returns (address) {
        IOTAToken t = new IOTAToken(
            "IOTA ERC-20",
            "IOTA",
            address(this),
            tokenId
        );
        _tokenAddresses[tokenId] = address(t);
        return address(t);
    }
}
