// Copyright 2020 IOTA Stiftung
// SPDX-License-Identifier: Apache-2.0

// (Re-)generated by schema tool
// >>>> DO NOT CHANGE THIS FILE! <<<<
// Change the json schema instead

import * as wasmtypes from "wasmlib/wasmtypes";

export const ScName        = "testcore";
export const ScDescription = "Wasm equivalent of built-in TestCore contract";
export const HScName       = new wasmtypes.ScHname(0x370d33ad);

export const ParamAddress         = "address";
export const ParamAgentID         = "agentID";
export const ParamCaller          = "caller";
export const ParamChainID         = "chainID";
export const ParamChainOwnerID    = "chainOwnerID";
export const ParamContractID      = "contractID";
export const ParamCounter         = "counter";
export const ParamFail            = "initFailParam";
export const ParamGasBudget       = "gasBudget";
export const ParamHash            = "Hash";
export const ParamHname           = "Hname";
export const ParamHnameContract   = "hnameContract";
export const ParamHnameEP         = "hnameEP";
export const ParamHnameZero       = "Hname-0";
export const ParamInt64           = "int64";
export const ParamInt64Zero       = "int64-0";
export const ParamIntValue        = "intParamValue";
export const ParamIotasWithdrawal = "iotasWithdrawal";
export const ParamN               = "n";
export const ParamName            = "intParamName";
export const ParamProgHash        = "progHash";
export const ParamString          = "string";
export const ParamStringZero      = "string-0";
export const ParamVarName         = "varName";

export const ResultChainOwnerID = "chainOwnerID";
export const ResultCounter      = "counter";
export const ResultN            = "n";
export const ResultSandboxCall  = "sandboxCall";
export const ResultValues       = "this";
export const ResultVars         = "this";

export const StateCounter = "counter";
export const StateInts    = "ints";
export const StateStrings = "strings";

export const FuncCallOnChain                 = "callOnChain";
export const FuncCheckContextFromFullEP      = "checkContextFromFullEP";
export const FuncClaimAllowance              = "claimAllowance";
export const FuncDoNothing                   = "doNothing";
export const FuncEstimateMinDust             = "estimateMinDust";
export const FuncIncCounter                  = "incCounter";
export const FuncInfiniteLoop                = "infiniteLoop";
export const FuncInit                        = "init";
export const FuncPassTypesFull               = "passTypesFull";
export const FuncPingAllowanceBack           = "pingAllowanceBack";
export const FuncRunRecursion                = "runRecursion";
export const FuncSendLargeRequest            = "sendLargeRequest";
export const FuncSendNFTsBack                = "sendNFTsBack";
export const FuncSendToAddress               = "sendToAddress";
export const FuncSetInt                      = "setInt";
export const FuncSpawn                       = "spawn";
export const FuncSplitFunds                  = "splitFunds";
export const FuncSplitFundsNativeTokens      = "splitFundsNativeTokens";
export const FuncTestBlockContext1           = "testBlockContext1";
export const FuncTestBlockContext2           = "testBlockContext2";
export const FuncTestCallPanicFullEP         = "testCallPanicFullEP";
export const FuncTestCallPanicViewEPFromFull = "testCallPanicViewEPFromFull";
export const FuncTestChainOwnerIDFull        = "testChainOwnerIDFull";
export const FuncTestEventLogDeploy          = "testEventLogDeploy";
export const FuncTestEventLogEventData       = "testEventLogEventData";
export const FuncTestEventLogGenericData     = "testEventLogGenericData";
export const FuncTestPanicFullEP             = "testPanicFullEP";
export const FuncWithdrawFromChain           = "withdrawFromChain";
export const ViewCheckContextFromViewEP      = "checkContextFromViewEP";
export const ViewFibonacci                   = "fibonacci";
export const ViewFibonacciIndirect           = "fibonacciIndirect";
export const ViewGetCounter                  = "getCounter";
export const ViewGetInt                      = "getInt";
export const ViewGetStringValue              = "getStringValue";
export const ViewInfiniteLoopView            = "infiniteLoopView";
export const ViewJustView                    = "justView";
export const ViewPassTypesView               = "passTypesView";
export const ViewTestCallPanicViewEPFromView = "testCallPanicViewEPFromView";
export const ViewTestChainOwnerIDView        = "testChainOwnerIDView";
export const ViewTestPanicViewEP             = "testPanicViewEP";
export const ViewTestSandboxCall             = "testSandboxCall";

export const HFuncCallOnChain                 = new wasmtypes.ScHname(0x95a3d123);
export const HFuncCheckContextFromFullEP      = new wasmtypes.ScHname(0xa56c24ba);
export const HFuncClaimAllowance              = new wasmtypes.ScHname(0x40bec0e6);
export const HFuncDoNothing                   = new wasmtypes.ScHname(0xdda4a6de);
export const HFuncEstimateMinDust             = new wasmtypes.ScHname(0xe700e7db);
export const HFuncIncCounter                  = new wasmtypes.ScHname(0x7b287419);
export const HFuncInfiniteLoop                = new wasmtypes.ScHname(0xf571430a);
export const HFuncInit                        = new wasmtypes.ScHname(0x1f44d644);
export const HFuncPassTypesFull               = new wasmtypes.ScHname(0x733ea0ea);
export const HFuncPingAllowanceBack           = new wasmtypes.ScHname(0x66f43c0b);
export const HFuncRunRecursion                = new wasmtypes.ScHname(0x833425fd);
export const HFuncSendLargeRequest            = new wasmtypes.ScHname(0xfdaaca3c);
export const HFuncSendNFTsBack                = new wasmtypes.ScHname(0x8f6ef428);
export const HFuncSendToAddress               = new wasmtypes.ScHname(0x63ce4634);
export const HFuncSetInt                      = new wasmtypes.ScHname(0x62056f74);
export const HFuncSpawn                       = new wasmtypes.ScHname(0xec929d12);
export const HFuncSplitFunds                  = new wasmtypes.ScHname(0xc7ea86c9);
export const HFuncSplitFundsNativeTokens      = new wasmtypes.ScHname(0x16532a28);
export const HFuncTestBlockContext1           = new wasmtypes.ScHname(0x796d4136);
export const HFuncTestBlockContext2           = new wasmtypes.ScHname(0x758b0452);
export const HFuncTestCallPanicFullEP         = new wasmtypes.ScHname(0x4c878834);
export const HFuncTestCallPanicViewEPFromFull = new wasmtypes.ScHname(0xfd7e8c1d);
export const HFuncTestChainOwnerIDFull        = new wasmtypes.ScHname(0x2aff1167);
export const HFuncTestEventLogDeploy          = new wasmtypes.ScHname(0x96ff760a);
export const HFuncTestEventLogEventData       = new wasmtypes.ScHname(0x0efcf939);
export const HFuncTestEventLogGenericData     = new wasmtypes.ScHname(0x6a16629d);
export const HFuncTestPanicFullEP             = new wasmtypes.ScHname(0x24fdef07);
export const HFuncWithdrawFromChain           = new wasmtypes.ScHname(0x405c0b0a);
export const HViewCheckContextFromViewEP      = new wasmtypes.ScHname(0x88ff0167);
export const HViewFibonacci                   = new wasmtypes.ScHname(0x7940873c);
export const HViewFibonacciIndirect           = new wasmtypes.ScHname(0x6dd98513);
export const HViewGetCounter                  = new wasmtypes.ScHname(0xb423e607);
export const HViewGetInt                      = new wasmtypes.ScHname(0x1887e5ef);
export const HViewGetStringValue              = new wasmtypes.ScHname(0xcf0a4d32);
export const HViewInfiniteLoopView            = new wasmtypes.ScHname(0x1a383295);
export const HViewJustView                    = new wasmtypes.ScHname(0x33b8972e);
export const HViewPassTypesView               = new wasmtypes.ScHname(0x1a5b87ea);
export const HViewTestCallPanicViewEPFromView = new wasmtypes.ScHname(0x91b10c99);
export const HViewTestChainOwnerIDView        = new wasmtypes.ScHname(0x26586c33);
export const HViewTestPanicViewEP             = new wasmtypes.ScHname(0x22bc4d72);
export const HViewTestSandboxCall             = new wasmtypes.ScHname(0x42d72b63);
