// // Copyright 2020 IOTA Stiftung
// // SPDX-License-Identifier: Apache-2.0

use std::collections::HashMap;
use std::sync::{Arc, Mutex};
use std::thread::spawn;
use std::time::Duration;

use crypto::signatures::ed25519::PublicKey;
use reqwest::{blocking, StatusCode};
use serde::{Deserialize, Serialize};
use wasmlib::*;
use ws::{CloseCode, connect, Message, Sender};

use crate::*;
use crate::codec::*;
use crate::keypair::KeyPair;

pub const ISC_EVENT_KIND_NEW_BLOCK: &str = "new_block";
pub const ISC_EVENT_KIND_RECEIPT: &str = "receipt";
pub const ISC_EVENT_KIND_SMART_CONTRACT: &str = "contract";
pub const ISC_EVENT_KIND_ERROR: &str = "error";

const READ_TIMEOUT: Duration = Duration::from_millis(10000);

#[derive(Serialize, Deserialize)]
pub struct SubscriptionCommand {
    pub command: String,
    pub topic: String,
}

#[derive(Serialize, Deserialize)]
pub struct EventMessage {
    #[serde(rename = "kind")]
    pub kind: String,
    #[serde(rename = "issuer")]
    pub issuer: String,
    #[serde(rename = "requestID")]
    pub request_id: String,
    #[serde(rename = "chainID")]
    pub chain_id: String,
    #[serde(rename = "payload")]
    pub payload: Vec<String>,
}

pub struct ContractEvent {
    pub chain_id: String,
    pub contract_id: String,
    pub data: String,
}

pub struct WasmClientService {
    chain_id: ScChainID,
    nonces: Arc<Mutex<HashMap<PublicKey, u64>>>,
    wasp_api: String,
}

impl WasmClientService {
    pub fn new(wasp_api: &str, chain_id: &str) -> Self {
        set_sandbox_wrappers(chain_id).unwrap();
        WasmClientService {
            chain_id: chain_id_from_string(chain_id),
            nonces: Arc::default(),
            wasp_api: String::from(wasp_api),
        }
    }

    pub(crate) fn call_view_by_hname(
        &self,
        contract_hname: &ScHname,
        function_hname: &ScHname,
        args: &[u8],
    ) -> Result<Vec<u8>> {
        let url = self.wasp_api.clone() + "/requests/callview";
        let client = blocking::Client::builder()
            .timeout(READ_TIMEOUT)
            .build()
            .unwrap();
        let body = APICallViewRequest {
            arguments: json_encode(args),
            chain_id: self.chain_id.to_string(),
            contract_hname: contract_hname.to_string(),
            function_hname: function_hname.to_string(),
        };
        let res = client.post(url).json(&body).send();
        match res {
            Ok(v) => match v.status() {
                StatusCode::OK => {
                    match v.json::<JsonResponse>() {
                        Ok(json_obj) => {
                            return Ok(json_decode(json_obj));
                        }
                        Err(e) => {
                            return Err(format!("call() response failed: {}", e.to_string()));
                        }
                    };
                }
                failed_status_code => {
                    let status_code = failed_status_code.as_u16();
                    match v.json::<JsonError>() {
                        Ok(err_msg) => {
                            return Err(format!("{}: {}", status_code, err_msg.message));
                        }
                        Err(e) => return Err(e.to_string()),
                    }
                }
            },
            Err(e) => {
                return Err(format!("call() request failed: {}", e.to_string()));
            }
        }
    }

    pub fn current_chain_id(&self) -> ScChainID {
        self.chain_id
    }

    pub(crate) fn post_request(
        &self,
        chain_id: &ScChainID,
        h_contract: &ScHname,
        h_function: &ScHname,
        args: &[u8],
        allowance: &ScAssets,
        key_pair: &keypair::KeyPair,
    ) -> Result<ScRequestID> {
        let nonce: u64;
        match self.cache_nonce(key_pair) {
            Ok(n) => nonce = n,
            Err(e) => return Err(e),
        }
        let mut req =
            offledgerrequest::OffLedgerRequest::new(
                chain_id,
                h_contract,
                h_function,
                args,
                nonce,
            );
        req.with_allowance(&allowance);
        let signed = req.sign(key_pair);

        let url = self.wasp_api.clone() + "/requests/offledger";
        let client = blocking::Client::new();
        let body = APIOffLedgerRequest {
            chain_id: chain_id.to_string(),
            request: hex_encode(&signed.to_bytes()),
        };
        let res = client.post(url).json(&body).send();
        match res {
            Ok(v) => match v.status() {
                StatusCode::OK => {}
                StatusCode::ACCEPTED => {}
                failed_status_code => {
                    let status_code = failed_status_code.as_u16();
                    match v.json::<JsonError>() {
                        Ok(err_msg) => {
                            return Err(format!("{}: {}", status_code, err_msg.message));
                        }
                        Err(e) => return Err(e.to_string()),
                    }
                }
            },
            Err(e) => {
                return Err(format!("post() request failed: {}", e.to_string()));
            }
        }
        Ok(signed.id())
    }

    fn subscribe(sender: &Sender, topic: &str) {
        let cmd = SubscriptionCommand {
            command: String::from("subscribe"),
            topic: String::from(topic),
        };
        let json = serde_json::to_string(&cmd).unwrap();
        let _ = sender.send(json);
    }

    pub(crate) fn subscribe_events(&self, event_handlers: Arc<Mutex<Vec<Box<dyn IEventHandlers>>>>, event_done: Arc<Mutex<bool>>) -> Result<()> {
        let socket_url = self.wasp_api.replace("http:", "ws:") + "/ws";
        spawn(move || {
            connect(socket_url, |out| {
                WasmClientService::subscribe(&out, "chains");
                WasmClientService::subscribe(&out, "block_events");
                let event_handlers = event_handlers.clone();
                let event_done = event_done.clone();
                move |msg: Message| {
                    println!("Message: {}", msg);
                    if let Ok(text) = msg.as_text() {
                        if let Ok(json) = serde_json::from_str::<EventMessage>(text) {
                            for item in json.payload {
                                let parts: Vec<String> = item.split(": ").map(|s| s.into()).collect();
                                let event = ContractEvent {
                                    chain_id: json.chain_id.clone(),
                                    contract_id: parts[0].clone(),
                                    data: parts[1].clone(),
                                };
                                WasmClientContext::process_event(&event_handlers, &event);
                            }
                        }
                    }
                    let done = *event_done.lock().unwrap();
                    if done {
                        return out.close(CloseCode::Normal);
                    }
                    return Ok(());
                }
            }).unwrap();
        });
        return Ok(());
    }

    pub(crate) fn wait_until_request_processed(
        &self,
        req_id: &ScRequestID,
        timeout: Duration,
    ) -> Result<()> {
        let url = format!(
            "{}/chains/{}/requests/{}/wait",
            self.wasp_api,
            self.chain_id.to_string(),
            req_id.to_string()
        );
        let client = blocking::Client::builder()
            .timeout(timeout)
            .build()
            .unwrap();
        let res = client.get(url).header("Content-Type", "application/json").send();
        match res {
            Ok(v) => match v.status() {
                StatusCode::OK => {
                    return Ok(());
                }
                failed_status_code => {
                    let status_code = failed_status_code.as_u16();
                    match v.text() {
                        Ok(err_msg) => {
                            return Err(format!("{}: {}", status_code, err_msg));
                        }
                        Err(e) => return Err(e.to_string()),
                    }
                }
            },
            Err(e) => {
                return Err(format!("request failed: {}", e.to_string()));
            }
        }
    }

    fn cache_nonce(&self, key_pair: &KeyPair) -> Result<u64> {
        let key = key_pair.public_key;
        let mut nonces = self.nonces.lock().unwrap();
        let mut nonce: u64;
        match nonces.get(&key) {
            None => {
                // get last used nonce from accounts core contract
                let isc_agent = ScAgentID::from_address(&key_pair.address());
                let chain_id = self.chain_id.to_string();
                let wcs = Arc::new(WasmClientService::new(&self.wasp_api, &chain_id));
                let ctx = WasmClientContext::new(wcs, coreaccounts::SC_NAME);
                let n = coreaccounts::ScFuncs::get_account_nonce(&ctx);
                n.params.agent_id().set_value(&isc_agent);
                n.func.call();
                match ctx.err() {
                    Ok(_) => nonce = n.results.account_nonce().value(),
                    Err(e) => return Err(e),
                }
            }
            Some(n) => nonce = *n,
        }
        nonce += 1;
        nonces.insert(key, nonce);
        Ok(nonce)
    }
}
