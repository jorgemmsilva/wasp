# wasp-solo

The `wasp-solo` tool emulates a Wasp node with Solo as in-memory L1 ledger,
replacing local-setup as a more lightweight alternative to test ISC and EVM
contracts.

## Example: Uniswap test suite

The following commands will clone and run the Uniswap contract tests against ISC's EVM.

Start the `wasp-solo` tool:

```
wasp-solo
```

In another terminal, clone uniswap:

```
git clone https://github.com/Uniswap/uniswap-v3-core.git
yarn install
npx hardhat compile
```

Edit `hardhat.config.ts`, section `networks`:

```
wasp: {
    chainId: 1074,
    url: 'http://localhost:9090/v1/chains/test1rr0vuvh0e5yhwfe0rtz6kzrhvfz3lntzyt2xtpc98372nz9kcljp5dzs5p5/evm',
},
```

Run the test suite:

```
npx hardhat test --network wasp
```
