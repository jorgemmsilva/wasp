const { expect } = require("chai");
const { ethers } = require("hardhat");

describe("ISCP", function () {
  const entropy1 =
    "0x0000000000000000000000000000000000000000000000000000000000000001";

  const entropy2 =
    "0x0000000000000000000000000000000000000000000000000000000000000002";

  it("Should return the correct chainId once initialized", async function () {
    const ISCPFactory = await ethers.getContractFactory("ISCP");
    const iscp = await ISCPFactory.deploy("0x1074");
    await iscp.deployed();

    expect(await iscp.chainId()).to.equal("0x1074");
  });

  it("Should allow entropy to be set", async function () {
    const ISCPFactory = await ethers.getContractFactory("ISCP");
    const iscp = await ISCPFactory.deploy("0x1074");
    await iscp.deployed();

    const setEntropy1Tx = await iscp.setEntropy(entropy1);
    await setEntropy1Tx.wait(); // wait for the tx to be mined
    expect(await iscp.entropy()).to.equal(entropy1);

    const setEntropy2Tx = await iscp.setEntropy(entropy2);
    await setEntropy2Tx.wait();
    expect(await iscp.entropy()).to.equal(entropy2);
  });

  it("Should stop accepting changes to entropy  after its locked for the current block", async function () {
    const ISCPFactory = await ethers.getContractFactory("ISCP");
    const iscp = await ISCPFactory.deploy("0x1074");
    await iscp.deployed();

    const setEntropy1Tx = await iscp.setEntropy(entropy1);
    const lockTx = await iscp.lockForBlock();
    const setEntropy2Tx = await iscp.setEntropy(entropy2);

    await Promise.all([
      setEntropy1Tx.wait(),
      lockTx.wait(),
      setEntropy2Tx.wait(),
    ]);

    expect(await iscp.entropy()).to.equal(entropy1);
  });

  it("Should allow tokens to be wrapped", async function () {
    // TODO
  });

  // TODO check it is not possible to wrap after locking

  // TODO check unwrap
  // TODO check it is possible to unwrap after locking
});
