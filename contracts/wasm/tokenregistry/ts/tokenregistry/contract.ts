// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

// (Re-)generated by schema tool
// >>>> DO NOT CHANGE THIS FILE! <<<<
// Change the json schema instead

import * as wasmlib from "wasmlib";
import * as sc from "./index";

export class InitCall {
	func: wasmlib.ScInitFunc;
	params: sc.MutableInitParams = new sc.MutableInitParams(wasmlib.ScView.nilProxy);
	public constructor(ctx: wasmlib.ScFuncCallContext) {
		this.func = new wasmlib.ScInitFunc(ctx, sc.HScName, sc.HFuncInit);
	}
}

export class InitContext {
	params: sc.ImmutableInitParams = new sc.ImmutableInitParams(wasmlib.paramsProxy());
	state: sc.MutableTokenRegistryState = new sc.MutableTokenRegistryState(wasmlib.ScState.proxy());
}

export class MintSupplyCall {
	func: wasmlib.ScFunc;
	params: sc.MutableMintSupplyParams = new sc.MutableMintSupplyParams(wasmlib.ScView.nilProxy);
	public constructor(ctx: wasmlib.ScFuncCallContext) {
		this.func = new wasmlib.ScFunc(ctx, sc.HScName, sc.HFuncMintSupply);
	}
}

export class MintSupplyContext {
	params: sc.ImmutableMintSupplyParams = new sc.ImmutableMintSupplyParams(wasmlib.paramsProxy());
	state: sc.MutableTokenRegistryState = new sc.MutableTokenRegistryState(wasmlib.ScState.proxy());
}

export class TransferOwnershipCall {
	func: wasmlib.ScFunc;
	params: sc.MutableTransferOwnershipParams = new sc.MutableTransferOwnershipParams(wasmlib.ScView.nilProxy);
	public constructor(ctx: wasmlib.ScFuncCallContext) {
		this.func = new wasmlib.ScFunc(ctx, sc.HScName, sc.HFuncTransferOwnership);
	}
}

export class TransferOwnershipContext {
	params: sc.ImmutableTransferOwnershipParams = new sc.ImmutableTransferOwnershipParams(wasmlib.paramsProxy());
	state: sc.MutableTokenRegistryState = new sc.MutableTokenRegistryState(wasmlib.ScState.proxy());
}

export class UpdateMetadataCall {
	func: wasmlib.ScFunc;
	params: sc.MutableUpdateMetadataParams = new sc.MutableUpdateMetadataParams(wasmlib.ScView.nilProxy);
	public constructor(ctx: wasmlib.ScFuncCallContext) {
		this.func = new wasmlib.ScFunc(ctx, sc.HScName, sc.HFuncUpdateMetadata);
	}
}

export class UpdateMetadataContext {
	params: sc.ImmutableUpdateMetadataParams = new sc.ImmutableUpdateMetadataParams(wasmlib.paramsProxy());
	state: sc.MutableTokenRegistryState = new sc.MutableTokenRegistryState(wasmlib.ScState.proxy());
}

export class GetInfoCall {
	func: wasmlib.ScView;
	params: sc.MutableGetInfoParams = new sc.MutableGetInfoParams(wasmlib.ScView.nilProxy);
	public constructor(ctx: wasmlib.ScViewCallContext) {
		this.func = new wasmlib.ScView(ctx, sc.HScName, sc.HViewGetInfo);
	}
}

export class GetInfoContext {
	params: sc.ImmutableGetInfoParams = new sc.ImmutableGetInfoParams(wasmlib.paramsProxy());
	state: sc.ImmutableTokenRegistryState = new sc.ImmutableTokenRegistryState(wasmlib.ScState.proxy());
}

export class ScFuncs {
	static init(ctx: wasmlib.ScFuncCallContext): InitCall {
		const f = new InitCall(ctx);
		f.params = new sc.MutableInitParams(wasmlib.newCallParamsProxy(f.func));
		return f;
	}

	static mintSupply(ctx: wasmlib.ScFuncCallContext): MintSupplyCall {
		const f = new MintSupplyCall(ctx);
		f.params = new sc.MutableMintSupplyParams(wasmlib.newCallParamsProxy(f.func));
		return f;
	}

	static transferOwnership(ctx: wasmlib.ScFuncCallContext): TransferOwnershipCall {
		const f = new TransferOwnershipCall(ctx);
		f.params = new sc.MutableTransferOwnershipParams(wasmlib.newCallParamsProxy(f.func));
		return f;
	}

	static updateMetadata(ctx: wasmlib.ScFuncCallContext): UpdateMetadataCall {
		const f = new UpdateMetadataCall(ctx);
		f.params = new sc.MutableUpdateMetadataParams(wasmlib.newCallParamsProxy(f.func));
		return f;
	}

	static getInfo(ctx: wasmlib.ScViewCallContext): GetInfoCall {
		const f = new GetInfoCall(ctx);
		f.params = new sc.MutableGetInfoParams(wasmlib.newCallParamsProxy(f.func));
		return f;
	}
}
