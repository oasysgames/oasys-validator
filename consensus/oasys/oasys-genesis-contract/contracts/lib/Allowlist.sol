// SPDX-License-Identifier: GPL-3.0

pragma solidity 0.8.12;

import "@openzeppelin/contracts/access/Ownable.sol";
import { IAllowlist } from "./IAllowlist.sol";

// Tried to add a zero address.
error EmptyAddress();

// Address already added.
error AlreadyAdded();

// Address not found.
error NotFound();

/**
 * @title Allowlist
 * @dev Allowlist manages the allowed addresses.
 * This contract allows all addresses after renouncing ownership.
 */
contract Allowlist is IAllowlist, Ownable {
    /*************
     * Variables *
     *************/

    address[] private _allowlist;
    mapping(address => uint256) private _ids;

    /********************
     * Public Functions *
     ********************/

    /**
     * Add the address into the allowlist.
     * @param _address Allowed address.
     */
    function addAddress(address _address) external onlyOwner {
        if (_address == address(0)) revert EmptyAddress();
        if (_contains(_address)) revert AlreadyAdded();
        _allowlist.push(_address);
        _ids[_address] = _allowlist.length;

        emit AllowlistAdded(_address);
    }

    /**
     * Remove the address from the allowlist.
     * @param _address Removed address.
     */
    function removeAddress(address _address) external onlyOwner {
        if (!_contains(_address)) revert NotFound();
        uint256 length = _allowlist.length;
        if (length > 1) {
            uint256 id = _ids[_address];
            address last = _allowlist[length - 1];
            _allowlist[id - 1] = last;
            _ids[last] = id;
        }
        _ids[_address] = 0;
        _allowlist.pop();

        emit AllowlistRemoved(_address);
    }

    /**
     * Returns the allowlist.
     */
    function getAllowlist() external view returns (address[] memory) {
        return _allowlist;
    }

    /**
     * Check if the allowlist contains the address.
     * @param _address Target address.
     */
    function containsAddress(address _address) external view returns (bool) {
        if (owner() == address(0)) {
            return true;
        }
        return _contains(_address);
    }

    /**********************
     * Internal Functions *
     **********************/

    /**
     * Check if the array of address contains the address.
     * @param _address address.
     * @return Contains of not.
     */
    function _contains(address _address) internal view returns (bool) {
        return _ids[_address] > 0;
    }
}
