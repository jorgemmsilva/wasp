// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

pragma solidity ^0.8.3;

import "@openzeppelin/contracts/token/ERC20/ERC20.sol";
import "./ERC20/IOTAFactory.sol";

address constant ISCP_CONTRACT_ADDRESS = 0x0000000000000000000000000000000000001074;

// The standard ISCP contract present in all EVM ISCP chains at ISCP_CONTRACT_ADDRESS
contract ISCP is IOTAFactory {
    // The ChainID of the underlying ISCP chain
    bytes public chainId;

    // result of calling ctx.GetEntropy(); automatically updated at the beginning of the ISCP block
    bytes32 public entropy;

    // block at which the contract stops accepting changes
    uint256 private _lockedBlock;

    // Throws if called after the contract was locked for the current block
    modifier unlocked() {
        require(block.number > _lockedBlock);
        _;
    }

    constructor(bytes memory _chainId) {
        chainId = _chainId;
    }

    // Locks the contract for the current block, so that no more changes are accepted
    function lockForBlock() public {
        _lockedBlock = block.number;
    }

    function setEntropy(bytes32 _entropy) public unlocked {
        entropy = _entropy;
    }

    // Wrapping logic

    event WrappedIOTAs(
        address indexed _cenas,
        uint256 amount,
        bytes indexed tokenId
    );

    function wrapIotas(
        bytes memory tokenId,
        address target,
        uint256 amount
    ) public unlocked {
        address tokenAddress = _tokenAddresses[tokenId];
        if (tokenAddress == address(0)) {
            // create a new ERC20 contract for this tokenId
            tokenAddress = deployNewToken(tokenId);
        }
        ERC20 token = ERC20(tokenAddress);
        require(token.transfer(target, amount));
        emit WrappedIOTAs(target, amount, tokenId);
    }

    // Unwrapping logic

    event UnwrappedIOTAs(address _cenas, uint256 amount, bytes indexed tokenId);

    // it is expected that the caller has "allowed" the ERC20 to be spent by the ISCP contract before calling to unwrap
    function unwrapIotas(address tokenAddress, uint256 amount) external {
        IOTAToken token = IOTAToken(tokenAddress);
        require(token.transferFrom(msg.sender, address(this), amount));
        emit UnwrappedIOTAs(msg.sender, amount, token.tokenId());
    }
}
