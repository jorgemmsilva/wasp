const { expect } = require("chai");
const { ethers } = require("hardhat");

describe("ISCP", function () {
  it("Should return the chainId once initialized", async function () {
    const ISCPFactory = await ethers.getContractFactory("ISCP");
    const iscp = await ISCPFactory.deploy(); // TODO HOW TO PASS CHAIN ID ???? or any custon struct
    await iscp.deployed();

    // expect(await iscp.greet()).to.equal("Hello, world!");

    // const setGreetingTx = await iscp.setGreeting("Hola, mundo!");

    // // wait until the transaction is mined
    // await setGreetingTx.wait();

    // expect(await iscp.greet()).to.equal("Hola, mundo!");
  });
});
