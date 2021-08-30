(self.webpackChunkdoc_ops=self.webpackChunkdoc_ops||[]).push([[415],{3905:function(t,e,a){"use strict";a.d(e,{Zo:function(){return l},kt:function(){return p}});var n=a(7294);function s(t,e,a){return e in t?Object.defineProperty(t,e,{value:a,enumerable:!0,configurable:!0,writable:!0}):t[e]=a,t}function i(t,e){var a=Object.keys(t);if(Object.getOwnPropertySymbols){var n=Object.getOwnPropertySymbols(t);e&&(n=n.filter((function(e){return Object.getOwnPropertyDescriptor(t,e).enumerable}))),a.push.apply(a,n)}return a}function o(t){for(var e=1;e<arguments.length;e++){var a=null!=arguments[e]?arguments[e]:{};e%2?i(Object(a),!0).forEach((function(e){s(t,e,a[e])})):Object.getOwnPropertyDescriptors?Object.defineProperties(t,Object.getOwnPropertyDescriptors(a)):i(Object(a)).forEach((function(e){Object.defineProperty(t,e,Object.getOwnPropertyDescriptor(a,e))}))}return t}function r(t,e){if(null==t)return{};var a,n,s=function(t,e){if(null==t)return{};var a,n,s={},i=Object.keys(t);for(n=0;n<i.length;n++)a=i[n],e.indexOf(a)>=0||(s[a]=t[a]);return s}(t,e);if(Object.getOwnPropertySymbols){var i=Object.getOwnPropertySymbols(t);for(n=0;n<i.length;n++)a=i[n],e.indexOf(a)>=0||Object.prototype.propertyIsEnumerable.call(t,a)&&(s[a]=t[a])}return s}var c=n.createContext({}),h=function(t){var e=n.useContext(c),a=e;return t&&(a="function"==typeof t?t(e):o(o({},e),t)),a},l=function(t){var e=h(t.components);return n.createElement(c.Provider,{value:e},t.children)},u={inlineCode:"code",wrapper:function(t){var e=t.children;return n.createElement(n.Fragment,{},e)}},d=n.forwardRef((function(t,e){var a=t.components,s=t.mdxType,i=t.originalType,c=t.parentName,l=r(t,["components","mdxType","originalType","parentName"]),d=h(a),p=s,f=d["".concat(c,".").concat(p)]||d[p]||u[p]||i;return a?n.createElement(f,o(o({ref:e},l),{},{components:a})):n.createElement(f,o({ref:e},l))}));function p(t,e){var a=arguments,s=e&&e.mdxType;if("string"==typeof t||s){var i=a.length,o=new Array(i);o[0]=d;var r={};for(var c in e)hasOwnProperty.call(e,c)&&(r[c]=e[c]);r.originalType=t,r.mdxType="string"==typeof t?t:s,o[1]=r;for(var h=2;h<i;h++)o[h]=a[h];return n.createElement.apply(null,o)}return n.createElement.apply(null,a)}d.displayName="MDXCreateElement"},4097:function(t,e,a){"use strict";a.r(e),a.d(e,{frontMatter:function(){return r},contentTitle:function(){return c},metadata:function(){return h},toc:function(){return l},default:function(){return d}});var n=a(2122),s=a(9756),i=(a(7294),a(3905)),o=["components"],r={},c="State, transitions and state anchoring",h={unversionedId:"guide/core_concepts/states",id:"guide/core_concepts/states",isDocsHomePage:!1,title:"State, transitions and state anchoring",description:"State of the chain",source:"@site/docs/guide/core_concepts/states.md",sourceDirName:"guide/core_concepts",slug:"/guide/core_concepts/states",permalink:"/docs/guide/core_concepts/states",editUrl:"https://github.com/iotaledger/chronicle.rs/tree/main/docs/docs/guide/core_concepts/states.md",version:"current",frontMatter:{},sidebar:"tutorialSidebar",previous:{title:"Consensus",permalink:"/docs/guide/core_concepts/consensus"},next:{title:"core-contracts",permalink:"/docs/guide/core_concepts/core-contracts"}},l=[{value:"State of the chain",id:"state-of-the-chain",children:[]},{value:"Digital assets on the chain",id:"digital-assets-on-the-chain",children:[]},{value:"The data state",id:"the-data-state",children:[]},{value:"Anchoring the State",id:"anchoring-the-state",children:[]},{value:"State Transitions",id:"state-transitions",children:[]}],u={toc:l};function d(t){var e=t.components,r=(0,s.Z)(t,o);return(0,i.kt)("wrapper",(0,n.Z)({},u,r,{components:e,mdxType:"MDXLayout"}),(0,i.kt)("h1",{id:"state-transitions-and-state-anchoring"},"State, transitions and state anchoring"),(0,i.kt)("h2",{id:"state-of-the-chain"},"State of the chain"),(0,i.kt)("p",null,"The state of the chain consists of:"),(0,i.kt)("ul",null,(0,i.kt)("li",{parentName:"ul"},"Balances of the native IOTA digital assets, colored tokens. The chain acts as a custodian for those funds"),(0,i.kt)("li",{parentName:"ul"},"A collection of arbitrary key/value pairs, the data state, which represents use case-specific data stored in the chain by its smart contracts outside of the UTXO ledger.")),(0,i.kt)("p",null,"The state of the chain is an append-only (immutable) data structure maintained by the distributed consensus of its validators."),(0,i.kt)("h2",{id:"digital-assets-on-the-chain"},"Digital assets on the chain"),(0,i.kt)("p",null,"The native L1 accounts of IOTA UTXO ledger are represented by addresses, each controlled by the entity holding the corresponding private/public key pair. The L1 account is a collection of UTXOs belonging to the address."),(0,i.kt)("p",null,"Similarly, the chain holds all tokens entrusted to it in one special UTXO, the state output (see above) which is always located in the address controlled by the chain.\nIt is similar to how a bank holds all deposits in its vault. This way, the chain (entity controlling the state output) becomes a custodian for the assets owned by its clients, in the same sense the bank\u2019s client owns the money deposited in the bank."),(0,i.kt)("p",null,"We call the consolidated assets held in the chain \u201ctotal assets on-chain\u201d, which are contained in the state output of the chain."),(0,i.kt)("h2",{id:"the-data-state"},"The data state"),(0,i.kt)("p",null,"The data state of the chain consists of the collection of key/value pairs. Each key and each value are arbitrary byte arrays."),(0,i.kt)("p",null,"In its persistent form, the data state is stored in the key/value database outside of the UTXO ledger and maintained by the validator nodes of the chain.\nThe state stored in the database is called a solid state."),(0,i.kt)("p",null,"The virtual state is a in-memory collection of key/value pairs which can become solid upon being committed to the database. An essential property of the virtual state is the possibility to have several virtual states in parallel as candidates, with a possibility for one of them to be solidified."),(0,i.kt)("p",null,"The data state in any form has: a state hash, timestamp and state index.\n(State hash is usually a Merkle root but it can be any hashing function of all data contained in the data state)"),(0,i.kt)("p",null,"The data state and the on-chain assets are both contained in one atomic unit on the ledger: the state UTXO. The state hash can only be changed by the same entity which controls the funds (the committee). So, the state mutation (state transition) of the chain is an atomic event between funds and the data state."),(0,i.kt)("h2",{id:"anchoring-the-state"},"Anchoring the State"),(0,i.kt)("p",null,"The data state is stored outside of the ledger, on the distributed database maintained by validators nodes."),(0,i.kt)("p",null,"By anchoring the state we mean: placing the hash of the data state into one special transaction and one special UTXO (an output) and adding it (confirming) on the UTXO ledger."),(0,i.kt)("p",null,"The UTXO ledger guarantees that at every moment there\u2019s ",(0,i.kt)("em",{parentName:"p"},"exactly one")," such output for each chain on the UTXO ledger. We call this output the ",(0,i.kt)("em",{parentName:"p"},"state output")," (or state anchor) and the containing transaction ",(0,i.kt)("em",{parentName:"p"},"state transaction")," (or anchor transaction) of the chain."),(0,i.kt)("p",null,"The state output is controlled (i.e. can be unlocked/consumed) by the entity running the chain."),(0,i.kt)("p",null,"With the anchoring mechanism the UTXO ledger supports the ISCP chain the following way:"),(0,i.kt)("ul",null,(0,i.kt)("li",{parentName:"ul"},"guarantees global consensus on the state of the chain"),(0,i.kt)("li",{parentName:"ul"},"makes the state immutable and tamper-proof"),(0,i.kt)("li",{parentName:"ul"},"makes the state consistent (see below)")),(0,i.kt)("p",null,"The state output contains:"),(0,i.kt)("ul",null,(0,i.kt)("li",{parentName:"ul"},"Identity of the chain (alias address)"),(0,i.kt)("li",{parentName:"ul"},"Hash of the data state"),(0,i.kt)("li",{parentName:"ul"},"State index, which is incremented with each next state output, the state transition (see below)")),(0,i.kt)("h2",{id:"state-transitions"},"State Transitions"),(0,i.kt)("p",null,"The Data state is updated by mutations of its key value pairs. Each mutation is either setting a value for a key, or deleting a key (and associeted value). Any update to the data state can be reduced to the partially ordered sequence of mutations."),(0,i.kt)("p",null,"The collection of mutations to the data state which is applied in a transition we call a ",(0,i.kt)("em",{parentName:"p"},"block"),":"),(0,i.kt)("pre",null,(0,i.kt)("code",{parentName:"pre"},"next data state = apply(current data state, block)\n")),(0,i.kt)("p",null,"The ",(0,i.kt)("em",{parentName:"p"},"state transition")," in the chain occurs atomically together with the movement of the chain's assets and the update of the state hash to the hash of the new data state in the transaction which consumes previous state output and produces next state output."),(0,i.kt)("p",null,"At any moment of time, the data state of the chain is a result of applying the historical sequence of blocks, starting from the empty data state. Hence, blockchain."),(0,i.kt)("p",null,(0,i.kt)("img",{alt:"state transitions",src:a(8716).Z})),(0,i.kt)("p",null,"On the UTXO ledger (L1), the history of the state is represented as a sequence (chain) of UTXOs, each holding chain\u2019s assets in a particular state and the anchoring hash of the data state. Note that not all of the state transitions history may be available: due to practical reasons the older transaction may be pruned in the snapshot process. The only thing that is guaranteed: the tip of the chain of UTXOs is always available (which includes the latest data state)."),(0,i.kt)("p",null,"The blocks and state outputs which anchor the state are computed by the Virtual Machine (VM), a deterministic processor, a \u201cblack box\u201d. The VM is responsible for the consistency of state transition and the state itself."),(0,i.kt)("p",null,(0,i.kt)("img",{alt:"chain",src:a(8714).Z})))}d.isMDXComponent=!0},8716:function(t,e,a){"use strict";e.Z=a.p+"assets/images/chain0-70ec82f973cafa300a093a6aa6073ac0.png"},8714:function(t,e,a){"use strict";e.Z=a.p+"assets/images/chain1-aa95a6d64b6dfecdb71d3280a7a624ae.png"}}]);