// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

// (Re-)generated by schema tool
// >>>> DO NOT CHANGE THIS FILE! <<<<
// Change the json schema instead

#![allow(dead_code)]
#![allow(unused_imports)]

use tokenregistry::*;
use wasmlib::*;

use crate::consts::*;
use crate::params::*;
use crate::state::*;
use crate::structs::*;

mod consts;
mod contract;
mod params;
mod state;
mod structs;

mod tokenregistry;

const EXPORT_MAP: ScExportMap = ScExportMap {
    names: &[
    	FUNC_INIT,
    	FUNC_MINT_SUPPLY,
    	FUNC_TRANSFER_OWNERSHIP,
    	FUNC_UPDATE_METADATA,
    	VIEW_GET_INFO,
	],
    funcs: &[
    	func_init_thunk,
    	func_mint_supply_thunk,
    	func_transfer_ownership_thunk,
    	func_update_metadata_thunk,
	],
    views: &[
    	view_get_info_thunk,
	],
};

#[no_mangle]
fn on_call(index: i32) {
	ScExports::call(index, &EXPORT_MAP);
}

#[no_mangle]
fn on_load() {
    ScExports::export(&EXPORT_MAP);
}

pub struct InitContext {
	params: ImmutableInitParams,
	state: MutableTokenRegistryState,
}

fn func_init_thunk(ctx: &ScFuncContext) {
	ctx.log("tokenregistry.funcInit");
	let f = InitContext {
		params: ImmutableInitParams { proxy: params_proxy() },
		state: MutableTokenRegistryState { proxy: state_proxy() },
	};
	func_init(ctx, &f);
	ctx.log("tokenregistry.funcInit ok");
}

pub struct MintSupplyContext {
	params: ImmutableMintSupplyParams,
	state: MutableTokenRegistryState,
}

fn func_mint_supply_thunk(ctx: &ScFuncContext) {
	ctx.log("tokenregistry.funcMintSupply");
	let f = MintSupplyContext {
		params: ImmutableMintSupplyParams { proxy: params_proxy() },
		state: MutableTokenRegistryState { proxy: state_proxy() },
	};
	func_mint_supply(ctx, &f);
	ctx.log("tokenregistry.funcMintSupply ok");
}

pub struct TransferOwnershipContext {
	params: ImmutableTransferOwnershipParams,
	state: MutableTokenRegistryState,
}

fn func_transfer_ownership_thunk(ctx: &ScFuncContext) {
	ctx.log("tokenregistry.funcTransferOwnership");
	let f = TransferOwnershipContext {
		params: ImmutableTransferOwnershipParams { proxy: params_proxy() },
		state: MutableTokenRegistryState { proxy: state_proxy() },
	};

	// TODO the one who can transfer token ownership
	let access = f.state.owner();
	ctx.require(access.exists(), "access not set: owner");
	ctx.require(ctx.caller() == access.value(), "no permission");

	ctx.require(f.params.token().exists(), "missing mandatory token");
	func_transfer_ownership(ctx, &f);
	ctx.log("tokenregistry.funcTransferOwnership ok");
}

pub struct UpdateMetadataContext {
	params: ImmutableUpdateMetadataParams,
	state: MutableTokenRegistryState,
}

fn func_update_metadata_thunk(ctx: &ScFuncContext) {
	ctx.log("tokenregistry.funcUpdateMetadata");
	let f = UpdateMetadataContext {
		params: ImmutableUpdateMetadataParams { proxy: params_proxy() },
		state: MutableTokenRegistryState { proxy: state_proxy() },
	};

	// TODO the one who can change the token info
	let access = f.state.owner();
	ctx.require(access.exists(), "access not set: owner");
	ctx.require(ctx.caller() == access.value(), "no permission");

	ctx.require(f.params.token().exists(), "missing mandatory token");
	func_update_metadata(ctx, &f);
	ctx.log("tokenregistry.funcUpdateMetadata ok");
}

pub struct GetInfoContext {
	params: ImmutableGetInfoParams,
	state: ImmutableTokenRegistryState,
}

fn view_get_info_thunk(ctx: &ScViewContext) {
	ctx.log("tokenregistry.viewGetInfo");
	let f = GetInfoContext {
		params: ImmutableGetInfoParams { proxy: params_proxy() },
		state: ImmutableTokenRegistryState { proxy: state_proxy() },
	};
	ctx.require(f.params.token().exists(), "missing mandatory token");
	view_get_info(ctx, &f);
	ctx.log("tokenregistry.viewGetInfo ok");
}
