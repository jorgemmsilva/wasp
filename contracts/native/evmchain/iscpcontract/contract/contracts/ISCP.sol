// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

pragma solidity ^0.8.3;

// import "hardhat/console.sol";
import "@openzeppelin/contracts/token/ERC20/ERC20.sol";
import "./ERC20/IOTAFactory.sol";

// ISCP addresses are 33 bytes which is sadly larger than EVM's bytes32 type, so
// it will use two 32-byte slots.
struct ISCPAddress {
    bytes1 typeId;
    bytes32 digest;
}

address constant ISCP_CONTRACT_ADDRESS = 0x0000000000000000000000000000000000001074;

// The standard ISCP contract present in all EVM ISCP chains at ISCP_CONTRACT_ADDRESS
contract ISCP is IOTAFactory {
    // The ChainID of the underlying ISCP chain
    ISCPAddress public chainId;

    // result of calling ctx.GetEntropy(); automatically updated at the beginning of the ISCP block
    bytes32 public entropy;

    // block at which the contract stops accepting changes
    uint256 private _lockedBlock;

    // Throws if called after the contract was locked for the current block
    modifier unlocked() {
        require(block.number > _lockedBlock);
        _;
    }

    constructor(string memory _chainId) {
        console.log("Deploying an ISCP contract with chainID:", chainId);
        chainId = _chainId;
    }

    // TODO check - getters should be generated automatically because those are public fields
    // function getChainId() public view returns (ISCPAddress memory) {
    //     return _chainId;
    // }

    // function getEntropy() public view returns (bytes32) {
    //     return entropy;
    // }

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
        TokenId indexed tokenId
    );

    function wrapIotas(
        TokenId memory tokenId,
        address memory target,
        uint256 memory amount
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

    event UnwrappedIOTAs(
        address indexed _cenas,
        uint256 amount,
        TokenId indexed tokenId
    );

    // it is expected that the caller has "allowed" the ERC20 to be spent by the ISCP contract before calling to unwrap
    function unwrapIotas(address memory tokenAddress, uint256 memory amount)
        external
    {
        ERC20 token = ERC20(tokenAddress);
        require(token.transferFrom(msg.sender, address(this), amount));
        emit UnwrappedIOTAs(msg.sender, amount, tokenId);
    }
}
