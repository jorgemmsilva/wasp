import type { IUnlockBlock } from './IUnlockBlock';
import type { Buffer } from '../buffer';
export interface ITransaction {
    /**
    * The transaction's version.
    */
    version: number;

    /**
     * The transaction's timestamp.
     */
    timestamp: bigint;

    /**
     * The nodeID to pledge access mana.
     */
    aManaPledge: string;

    /**
    * The nodeID to pledge consensus mana.
    */
    cManaPledge: string;

    /**
     * The inputs to send.
     */
    inputs: string[];

    payload: Buffer;

    chainId: string;
    /**
     * The outputs to send.
     */
    outputs: {
        [address: string]: {
            /**
             * The color.
             */
            color: string;
            /**
             * The value.
             */
            value: bigint;

            shouldShipPayload: boolean;
        }[];
    };

    /**
     * The signatures to send.
     */
    unlockBlocks: IUnlockBlock[];
}
