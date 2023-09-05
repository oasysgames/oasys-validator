// SPDX-License-Identifier: MIT
pragma solidity 0.8.12;

import "@openzeppelin/contracts/token/ERC20/ERC20.sol";

// Invalid mint destinaition.
error InvalidDestination();

// Already claimer address.
error AlreadyClaimer();

// Invalid since or until.
error InvalidClaimPeriod();

// OAS is zero.
error NoAmount();

// Invalid minter address.
error InvalidMinter();

// Over claimable OAS.
error OverAmount();

// OAS transfer failed.
error TransferFailed();

// Cannot renounce.
error CannotRenounce();

// Only staking contracts or burn are allowed.
error UnauthorizedTransfer();

/**
 * @title SOAS
 * @dev The SOAS is non-transferable but stakable token.
 * It is possible to gradually convert from since to until period to OAS.
 */
contract SOAS is ERC20 {
    /**********
     * Struct *
     **********/

    struct ClaimInfo {
        uint256 amount;
        uint256 claimed;
        uint64 since;
        uint64 until;
        address from;
    }

    /**********************
     * Contract Variables *
     **********************/

    address[] public allowedAddresses;
    mapping(address => ClaimInfo) public claimInfo;
    mapping(address => address) public originalClaimer;

    /**********
     * Events *
     **********/

    event Mint(address indexed to, uint256 amount, uint256 since, uint256 until);
    event Claim(address indexed holder, uint256 amount);
    event Renounce(address indexed holder, uint256 amount);
    event Allow(address indexed original, address indexed transferable);

    /***************
     * Constructor *
     ***************/

    /**
     * @param _allowedAddresses List of the preallowed contract address.
     */
    constructor(address[] memory _allowedAddresses) ERC20("Stakable OAS", "SOAS") {
        allowedAddresses = _allowedAddresses;
    }

    /********************
     * Public Functions *
     ********************/

    /**
     * Mint the SOAS by depositing the OAS.
     * @param to Destination address for the SOAS.
     * @param since Unixtime to start converting the SOAS to the OAS.
     * @param until Unixtime when all the SOAS can be converted to the OAS
     */
    function mint(
        address to,
        uint64 since,
        uint64 until
    ) external payable {
        if (to == address(0) || _contains(allowedAddresses, to)) revert InvalidDestination();
        if (originalClaimer[to] != address(0)) revert AlreadyClaimer();
        if (since <= block.timestamp || since >= until) revert InvalidClaimPeriod();
        if (msg.value == 0) revert NoAmount();

        _mint(to, msg.value);
        claimInfo[to] = ClaimInfo(msg.value, 0, since, until, msg.sender);
        originalClaimer[to] = to;

        emit Mint(to, msg.value, since, until);
    }

    /**
     * Allow the transferable address for the claimer address. 
     * @param original Address of the claimer.
     * @param allowed Transferable address.
     */
    function allow(address original, address allowed) external {
        if (claimInfo[original].from != msg.sender) revert InvalidMinter();
        if (originalClaimer[allowed] != address(0)) revert AlreadyClaimer();

        originalClaimer[allowed] = original;

        emit Allow(original, allowed);
    }

    /**
     * Convert the SOAS to the OAS.
     * @param amount Amount of the SOAS.
     */
    function claim(uint256 amount) external {
        if (amount == 0) revert NoAmount();

        ClaimInfo storage originalClaimInfo = claimInfo[originalClaimer[msg.sender]];
        uint256 currentClaimableOAS = getClaimableOAS(originalClaimer[msg.sender]) -
            originalClaimInfo.claimed;
        if (amount > currentClaimableOAS) revert OverAmount();

        originalClaimInfo.claimed += amount;

        _burn(msg.sender, amount);
        (bool success, ) = msg.sender.call{ value: amount }("");
        if (!success) revert TransferFailed();

        emit Claim(originalClaimer[msg.sender], amount);
    }

    /**
     * Return the SOAS as the OAS to the address that minted it.
     * @param amount Amount of the SOAS.
     */
    function renounce(uint256 amount) external {
        if (amount == 0) revert NoAmount();

        ClaimInfo storage originalClaimInfo = claimInfo[originalClaimer[msg.sender]];
        if (amount > originalClaimInfo.amount - originalClaimInfo.claimed) revert OverAmount();

        _burn(msg.sender, amount);
        (bool success, ) = originalClaimInfo.from.call{ value: amount }("");
        if (!success) revert TransferFailed();

        emit Renounce(originalClaimer[msg.sender], amount);
    }

    /**
     * Get current amount of the SOAS available for conversion.
     * @param original Holder of the SOAS token.
     */
    function getClaimableOAS(address original) public view returns (uint256) {
        ClaimInfo memory originalClaimInfo = claimInfo[original];
        if (originalClaimInfo.amount == 0) {
            return 0;
        }
        if (block.timestamp < originalClaimInfo.since) {
            return 0;
        }
        uint256 amount = (originalClaimInfo.amount * (block.timestamp - originalClaimInfo.since)) /
            (originalClaimInfo.until - originalClaimInfo.since);
        if (amount > originalClaimInfo.amount) {
            return originalClaimInfo.amount;
        }
        return amount;
    }

    /**********************
     * Internal Functions *
     **********************/

    /**
     * The SOAS is allowed to mint, burn and transfer with the Staking contract.
     */
    function _beforeTokenTransfer(
        address from,
        address to,
        uint256 amount
    ) internal view override {
        if (from == address(0) || to == address(0)) return;
        if (_contains(allowedAddresses, from) || _contains(allowedAddresses, to)) return;
        if (originalClaimer[from] == originalClaimer[to]) return;

        revert UnauthorizedTransfer();
    }

    /**
     * Whether list of the address contains the item address.
     */
    function _contains(address[] memory list, address item) internal pure returns (bool) {
        for (uint256 index = 0; index < list.length; index++) {
            if (list[index] == item) {
                return true;
            }
        }
        return false;
    }
}
