// SPDX-License-Identifier: GPL-3.0

pragma solidity 0.8.12;

// The contract has already been initialized.
error AlreadyInitialized();

// Sender must be block producer.
error OnlyBlockProducer();

/**
 * @title System
 */
abstract contract System {
    bool public initialized;

    /**
     * @dev Modifier requiring the initialize function to be called only once.
     */
    modifier initializer() {
        if (initialized) revert AlreadyInitialized();
        initialized = true;
        _;
    }

    /**
     * @dev Modifier requiring the sender to be validator of created this block.
     */
    modifier onlyCoinbase() {
        if (msg.sender != block.coinbase) revert OnlyBlockProducer();
        _;
    }
}
