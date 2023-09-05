// SPDX-License-Identifier: GPL-3.0

pragma solidity 0.8.12;

import { System } from "./System.sol";
import { IStakeManager } from "./IStakeManager.sol";
import { IEnvironment } from "./IEnvironment.sol";
import { IAllowlist } from "./lib/IAllowlist.sol";
import { UpdateHistories } from "./lib/UpdateHistories.sol";
import { Validator as ValidatorLib } from "./lib/Validator.sol";
import { Staker as StakerLib } from "./lib/Staker.sol";
import { Token } from "./lib/Token.sol";

// Only executable in the first block of epoch.
error OnlyFirstBlock();

// Only executable in the last block of epoch.
error OnlyLastBlock();

// Not executable in the last block of epoch.
error OnlyNotLastBlock();

// Validator does not exist.
error ValidatorDoesNotExist();

// Staker does not exist.
error StakerDoesNotExist();

// Unauthorized transaction sender.
error UnauthorizedSender();

// Unauthorized validator.
error UnauthorizedValidator();

// Amount or msg.value is zero.
error NoAmount();

/**
 * @title StakeManager
 * @dev The StakeManager contract is the core contract of the proof-of-stake.
 *
 */
contract StakeManager is IStakeManager, System {
    using UpdateHistories for uint256[];
    using ValidatorLib for Validator;
    using StakerLib for Staker;

    /*************
     * Constants *
     *************/

    IEnvironment public environment;
    IAllowlist public allowlist;

    /*************
     * Variables *
     *************/

    // Stake updated epochs
    uint256[] public stakeUpdates;
    // Stake amounts per epoch
    uint256[] public stakeAmounts;
    // List of validators
    mapping(address => Validator) public validators;
    address[] public validatorOwners;
    // Mapping of validator operator to validator owner
    mapping(address => address) public operatorToOwner;
    // List of stakers
    mapping(address => Staker) public stakers;
    address[] public stakerSigners;

    /*************
     * Modifiers *
     *************/

    /**
     * Modifier requiring the validator to be registered.
     * @param validator Validator address.
     */
    modifier validatorExists(address validator) {
        if (validators[validator].owner == address(0)) {
            revert ValidatorDoesNotExist();
        }
        _;
    }

    /**
     * Modifier requiring the sender to be a registered staker.
     */
    modifier stakerExists(address staker) {
        if (stakers[staker].signer == address(0)) {
            revert StakerDoesNotExist();
        }
        _;
    }

    /**
     * Modifier requiring the sender to be a owner or operator of the validator.
     * @param validator Validator address.
     */
    modifier onlyValidatorOwnerOrOperator(address validator) {
        Validator storage _validator = validators[validator];
        if (msg.sender != _validator.owner && msg.sender != _validator.operator) {
            revert UnauthorizedSender();
        }
        _;
    }

    /**
     * Modifier requiring the current block to be the first block of the epoch.
     */
    modifier onlyFirstBlock() {
        if (!environment.isFirstBlock()) revert OnlyFirstBlock();
        _;
    }

    /**
     * Modifier requiring the current block to be the last block of the epoch.
     */
    modifier onlyLastBlock() {
        if (!environment.isLastBlock()) revert OnlyLastBlock();
        _;
    }

    /**
     * Modifier requiring the current block not to be the last block of the epoch.
     */
    modifier onlyNotLastBlock() {
        if (environment.isLastBlock()) revert OnlyNotLastBlock();
        _;
    }

    /****************************
     * Functions for Validators *
     ****************************/

    /**
     * @inheritdoc IStakeManager
     */
    function initialize(IEnvironment _environment, IAllowlist _allowlist) external onlyCoinbase initializer {
        environment = _environment;
        allowlist = _allowlist;
    }

    /**
     * @inheritdoc IStakeManager
     */
    function slash(address operator, uint256 blocks) external validatorExists(operatorToOwner[operator]) onlyCoinbase {
        Validator storage validator = validators[operatorToOwner[operator]];
        uint256 until = validator.slash(environment.value(), environment.epoch(), blocks);
        emit ValidatorSlashed(validator.owner);
        if (until > 0) {
            emit ValidatorJailed(validator.owner, until);
        }
    }

    /*********************************************
     * Functions for Validator owner or operator *
     *********************************************/

    /**
     * @inheritdoc IStakeManager
     */
    function joinValidator(address operator) external {
        if (!allowlist.containsAddress(msg.sender)) {
            revert UnauthorizedValidator();
        }

        validators[msg.sender].join(operator);
        validatorOwners.push(msg.sender);
        operatorToOwner[operator] = msg.sender;
    }

    /**
     * @inheritdoc IStakeManager
     */
    function updateOperator(address operator) external validatorExists(msg.sender) {
        validators[msg.sender].updateOperator(operator);
        operatorToOwner[operator] = msg.sender;
    }

    /**
     * @inheritdoc IStakeManager
     */
    function activateValidator(address validator, uint256[] memory epochs)
        external
        onlyValidatorOwnerOrOperator(validator)
        validatorExists(validator)
        onlyNotLastBlock
    {
        validators[validator].activate(environment.epoch(), epochs);
        emit ValidatorActivated(validator, epochs);
    }

    /**
     * @inheritdoc IStakeManager
     */
    function deactivateValidator(address validator, uint256[] memory epochs)
        external
        onlyValidatorOwnerOrOperator(validator)
        validatorExists(validator)
        onlyNotLastBlock
    {
        validators[validator].deactivate(environment.epoch(), epochs);
        emit ValidatorDeactivated(validator, epochs);
    }

    /**
     * @inheritdoc IStakeManager
     */
    function claimCommissions(address validator, uint256 epochs) external validatorExists(validator) {
        validators[validator].claimCommissions(environment, epochs);
    }

    /************************
     * Functions for Staker *
     ************************/

    /**
     * @inheritdoc IStakeManager
     */
    function stake(
        address validator,
        Token.Type token,
        uint256 amount
    ) external payable validatorExists(validator) onlyNotLastBlock {
        if (amount == 0) revert NoAmount();

        stakeUpdates.add(stakeAmounts, environment.epoch() + 1, amount);

        Token.receives(token, msg.sender, amount);
        Staker storage staker = stakers[msg.sender];
        if (staker.signer == address(0)) {
            staker.signer = msg.sender;
            stakerSigners.push(msg.sender);
        }
        staker.stake(environment, validators[validator], token, amount);
        emit Staked(msg.sender, validator, token, amount);
    }

    /**
     * @inheritdoc IStakeManager
     */
    function unstake(
        address validator,
        Token.Type token,
        uint256 amount
    ) external validatorExists(validator) stakerExists(msg.sender) onlyNotLastBlock {
        if (amount == 0) revert NoAmount();

        stakeUpdates.sub(stakeAmounts, environment.epoch() + 1, amount);

        amount = stakers[msg.sender].unstake(environment, validators[validator], token, amount);
        emit Unstaked(msg.sender, validator, token, amount);
    }

    /**
     * @inheritdoc IStakeManager
     */
    function claimUnstakes(address staker) external stakerExists(staker) {
        stakers[staker].claimUnstakes(environment);
    }

    /**
     * @inheritdoc IStakeManager
     */
    function claimRewards(
        address staker,
        address validator,
        uint256 epochs
    ) external validatorExists(validator) stakerExists(staker) {
        stakers[staker].claimRewards(environment, validators[validator], epochs);
    }

    /******************
     * View Functions *
     ******************/

    /**
     * @inheritdoc IStakeManager
     */
    function getValidators(
        uint256 epoch,
        uint256 cursor,
        uint256 howMany
    )
        external
        view
        returns (
            address[] memory owners,
            address[] memory operators,
            uint256[] memory stakes,
            bool[] memory candidates,
            uint256 newCursor
        )
    {
        uint256 currentEpoch = environment.epoch();
        epoch = epoch > 0 ? epoch : currentEpoch;
        IEnvironment.EnvironmentValue memory env = environment.findValue(epoch);

        (howMany, newCursor) = _pagination(cursor, howMany, validatorOwners.length);
        owners = new address[](howMany);
        operators = new address[](howMany);
        stakes = new uint256[](howMany);
        candidates = new bool[](howMany);

        for (uint256 i = 0; i < howMany; i++) {
            Validator storage validator = validators[validatorOwners[cursor + i]];
            owners[i] = validator.owner;
            operators[i] = validator.operator;
            stakes[i] = validator.getTotalStake(epoch);
            candidates[i] =
                !validator.isInactive(epoch) &&
                !validator.isJailed(epoch) &&
                stakes[i] >= env.validatorThreshold;
        }

        return (owners, operators, stakes, candidates, newCursor);
    }

    /**
     * @inheritdoc IStakeManager
     */
    function getValidatorOwners(uint256 cursor, uint256 howMany)
        external
        view
        returns (address[] memory owners, uint256 newCursor)
    {
        (howMany, newCursor) = _pagination(cursor, howMany, validatorOwners.length);
        owners = new address[](howMany);
        for (uint256 i = 0; i < howMany; i++) {
            owners[i] = validatorOwners[cursor + i];
        }
        return (owners, newCursor);
    }

    /**
     * @inheritdoc IStakeManager
     */
    function getStakers(uint256 cursor, uint256 howMany)
        external
        view
        returns (address[] memory _stakers, uint256 newCursor)
    {
        (howMany, newCursor) = _pagination(cursor, howMany, stakerSigners.length);
        _stakers = new address[](howMany);
        for (uint256 i = 0; i < howMany; i++) {
            _stakers[i] = stakerSigners[cursor + i];
        }
        return (_stakers, newCursor);
    }

    /**
     * @inheritdoc IStakeManager
     */
    function getValidatorInfo(address validator, uint256 epoch)
        external
        view
        returns (
            address operator,
            bool active,
            bool jailed,
            bool candidate,
            uint256 stakes
        )
    {
        uint256 currentEpoch = environment.epoch();
        epoch = epoch > 0 ? epoch : currentEpoch;
        IEnvironment.EnvironmentValue memory env = environment.findValue(epoch);

        Validator storage _validator = validators[validator];
        active = !_validator.isInactive(epoch);
        jailed = _validator.isJailed(epoch);
        stakes = _validator.getTotalStake(epoch);
        candidate = active && !jailed && stakes >= env.validatorThreshold;

        return (_validator.operator, active, jailed, candidate, stakes);
    }

    /**
     * @inheritdoc IStakeManager
     */
    function getCommissions(address validator, uint256 epochs) external view returns (uint256 commissions) {
        (commissions, ) = validators[validator].getCommissions(environment, epochs);
        return commissions;
    }

    /**
     * @inheritdoc IStakeManager
     */
    function getUnstakes(address staker)
        external
        view
        returns (
            uint256 oasUnstakes,
            uint256 woasUnstakes,
            uint256 soasUnstakes
        )
    {
        Staker storage _staker = stakers[staker];
        oasUnstakes = _staker.getUnstakes(environment, Token.Type.OAS);
        woasUnstakes = _staker.getUnstakes(environment, Token.Type.wOAS);
        soasUnstakes = _staker.getUnstakes(environment, Token.Type.sOAS);

        return (oasUnstakes, woasUnstakes, soasUnstakes);
    }

    /**
     * @inheritdoc IStakeManager
     */
    function getRewards(
        address staker,
        address validator,
        uint256 epochs
    ) external view returns (uint256 rewards) {
        (rewards, ) = stakers[staker].getRewards(environment, validators[validator], epochs);
        return rewards;
    }

    /**
     * @inheritdoc IStakeManager
     */
    function getTotalStake(uint256 epoch) external view returns (uint256 amounts) {
        epoch = epoch > 0 ? epoch : environment.epoch();
        amounts = stakeUpdates.find(stakeAmounts, epoch);
        return amounts;
    }

    /**
     * @inheritdoc IStakeManager
     */
    function getTotalRewards(address[] memory _validators, uint256 epochs) external view returns (uint256 rewards) {
        uint256 epoch = environment.epoch() - epochs - 1;

        uint256 length = _validators.length;
        for (uint256 i = 0; i < epochs; i++) {
            epoch += 1;
            IEnvironment.EnvironmentValue memory env = environment.findValue(epoch);
            for (uint256 j = 0; j < length; j++) {
                rewards += validators[_validators[j]].getRewards(env, epoch);
            }
        }

        return rewards;
    }

    /**
     * @inheritdoc IStakeManager
     */
    function getValidatorStakes(
        address validator,
        uint256 epoch,
        uint256 cursor,
        uint256 howMany
    )
        external
        view
        returns (
            address[] memory _stakers,
            uint256[] memory stakes,
            uint256 newCursor
        )
    {
        Validator storage _validator = validators[validator];
        epoch = epoch > 0 ? epoch : environment.epoch();

        (howMany, newCursor) = _pagination(cursor, howMany, _validator.stakers.length);
        _stakers = new address[](howMany);
        stakes = new uint256[](howMany);

        for (uint256 i = 0; i < howMany; i++) {
            Staker storage staker = stakers[_validator.stakers[cursor + i]];
            _stakers[i] = staker.signer;
            stakes[i] =
                staker.getStake(_validator.owner, Token.Type.OAS, epoch) +
                staker.getStake(_validator.owner, Token.Type.wOAS, epoch) +
                staker.getStake(_validator.owner, Token.Type.sOAS, epoch);
        }

        return (_stakers, stakes, newCursor);
    }

    /**
     * @inheritdoc IStakeManager
     */
    function getStakerStakes(
        address staker,
        uint256 epoch,
        uint256 cursor,
        uint256 howMany
    )
        external
        view
        returns (
            address[] memory _validators,
            uint256[] memory oasStakes,
            uint256[] memory woasStakes,
            uint256[] memory soasStakes,
            uint256 newCursor
        )
    {
        Staker storage _staker = stakers[staker];
        epoch = epoch > 0 ? epoch : environment.epoch();

        (howMany, newCursor) = _pagination(cursor, howMany, validatorOwners.length);
        _validators = new address[](howMany);
        oasStakes = new uint256[](howMany);
        woasStakes = new uint256[](howMany);
        soasStakes = new uint256[](howMany);

        for (uint256 i = 0; i < howMany; i++) {
            _validators[i] = validatorOwners[cursor + i];
            oasStakes[i] = _staker.getStake(_validators[i], Token.Type.OAS, epoch);
            woasStakes[i] = _staker.getStake(_validators[i], Token.Type.wOAS, epoch);
            soasStakes[i] = _staker.getStake(_validators[i], Token.Type.sOAS, epoch);
        }

        return (_validators, oasStakes, woasStakes, soasStakes, newCursor);
    }

    /**
     * @inheritdoc IStakeManager
     */
    function getBlockAndSlashes(address validator, uint256 epoch)
        external
        view
        returns (uint256 blocks, uint256 slashes)
    {
        (blocks, slashes) = validators[validator].getBlockAndSlashes(epoch > 0 ? epoch : environment.epoch());
    }

    /*********************
     * Private Functions *
     *********************/

    function _pagination(
        uint256 cursor,
        uint256 howMany,
        uint256 length
    ) internal pure returns (uint256, uint256) {
        if (cursor + howMany >= length) {
            howMany = length - cursor;
        }
        return (howMany, cursor + howMany);
    }
}
