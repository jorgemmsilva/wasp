// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

pragma solidity ^0.8.3;

import "@openzeppelin/contracts/token/ERC20/ERC20.sol";

// TODO check how TokenId should be defined
struct TokenId {
    bytes1 typeId;
    bytes32 digest;
}

uint256 constant totalIOTASupply = 2779530283277761; // IOTA max supply

contract IOTAToken is ERC20 {
    TokenId public tokenId;

    constructor(
        string memory name,
        string memory symbol,
        address issuer,
        TokenId memory _tokenId
    ) ERC20(name, symbol) {
        tokenId = _tokenId;
        // mint the entire supply to the issuer (the ISCP factory contract)
        _mint(issuer, totalIOTASupply);
    }
}
