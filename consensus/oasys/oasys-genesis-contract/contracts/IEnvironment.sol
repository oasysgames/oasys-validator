// SPDX-License-Identifier: GPL-3.0

pragma solidity 0.8.12;

/**
 * @title IEnvironment
 */
interface IEnvironment {
    /***********
     * Structs *
     ***********/

    struct EnvironmentValue {
        // Block and epoch to which this setting applies
        uint256 startBlock;
        uint256 startEpoch;
        // Block generation interval(by seconds)
        uint256 blockPeriod;
        // Number of blocks in epoch
        uint256 epochPeriod;
        // Annual rate of staking reward
        uint256 rewardRate;
        // Validator commission rate
        uint256 commissionRate;
        // Amount of tokens required to become a validator
        uint256 validatorThreshold;
        // Number of not sealed to jailing the validator
        uint256 jailThreshold;
        // Number of epochs to jailing the validator
        uint256 jailPeriod;
    }

    /****************************
     * Functions for Validators *
     ****************************/

    /**
     * Initialization of contract.
     * This method is called by the genesis validator in the first epoch.
     * @param initialValue Initial environment value.
     */
    function initialize(EnvironmentValue memory initialValue) external;

    /**
     * Set the new environment value.
     * This method can only be called by validator, and the values are validated by other validators.
     * The new settings are applied starting at the epoch specified by "startEpoch".
     * @param newValue New environment value.
     */
    function updateValue(EnvironmentValue memory newValue) external;

    /******************
     * View Functions *
     ******************/

    /**
     * Returns the current epoch number.
     * @return Current epoch number.
     */
    function epoch() external view returns (uint256);

    /**
     * Determine if the current block is the first block of the epoch.
     * @return If true, it is the first block of the epoch.
     */
    function isFirstBlock() external view returns (bool);

    /**
     * Determine if the current block is the last block of the epoch.
     * @return If true, it is the last block of the epoch.
     */
    function isLastBlock() external view returns (bool);

    /**
     * Returns the environment value at the current epoch
     * @return Environment value.
     */
    function value() external view returns (EnvironmentValue memory);

    /**
     * Returns the environment value for the next epoch.
     * @return Environment value.
     */
    function nextValue() external view returns (EnvironmentValue memory);

    /**
     * Returns the environment value for the specific epoch.
     * @return Environment value.
     */
    function findValue(uint256 _epoch) external view returns (EnvironmentValue memory);
}
