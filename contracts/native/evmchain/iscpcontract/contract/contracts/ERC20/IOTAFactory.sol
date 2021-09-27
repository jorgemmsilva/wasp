// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

pragma solidity ^0.8.3;

import "./IOTAToken.sol";

contract IOTAFactory {
    mapping(TokenId => address) private _tokenAddresses;

    // event TokenCreated(address tokenAddress);

    function deployNewToken(TokenId memory tokenId) internal returns (address) {
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
