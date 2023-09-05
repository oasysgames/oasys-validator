// SPDX-License-Identifier: MIT
pragma solidity 0.8.12;

import { ECDSA } from "@openzeppelin/contracts/utils/cryptography/ECDSA.sol";
import { BytesLib } from "solidity-bytes-utils/contracts/BytesLib.sol";

contract Signers {
    /**********
     * Events *
     **********/

    event SignerAdded(address indexed _address);
    event SignerRemoved(address indexed _address);
    event ThresholdUpdated(uint256 indexed _threshold);

    /**********************
     * Contract Variables *
     **********************/

    uint256 public nonce;
    uint256 public threshold;
    address[] private _signers;
    mapping(address => uint256) private _signerId;

    /***************
     * Constructor *
     ***************/

    constructor(address[] memory signers, uint256 _threshold) {
        for (uint256 i = 0; i < signers.length; i++) {
            address signer = signers[i];
            require(signer != address(0), "Signer is zero address.");
            require(!_contains(signer), "Duplicate signer.");

            _signers.push(signer);
            _signerId[signer] = _signers.length;

            emit SignerAdded(signer);
        }

        require(_threshold > 0, "Threshold is zero.");
        require(_signers.length >= _threshold, "Signer shortage.");
        threshold = _threshold;

        emit ThresholdUpdated(_threshold);
    }

    /********************
     * Public Functions *
     ********************/

    function verifySignatures(
        bytes32 _hash,
        uint64 expiration,
        bytes memory signatures
    ) public view returns (bool) {
        require(_hash != 0x0, "Hash is empty");
        require(expiration >= block.timestamp, "Signature expired");
        require(signatures.length % 65 == 0, "Invalid signatures length");

        uint256 signatureCount = signatures.length / 65;
        uint256 signerCount = 0;
        address lastSigner = address(0);
        uint256 chainid = block.chainid;
        for (uint256 i = 0; i < signatureCount; i++) {
            address _signer = _recoverSigner(
                _hash,
                chainid,
                expiration,
                signatures,
                i * 65
            );
            if (_contains(_signer)) {
                signerCount++;
            }

            require(_signer > lastSigner, "Invalid address sort");
            lastSigner = _signer;
        }

        return signerCount >= threshold;
    }

    function _recoverSigner(
        bytes32 _hash,
        uint256 chainid,
        uint64 expiration,
        bytes memory signatures,
        uint256 index
    ) private pure returns (address) {
        require(signatures.length >= index + 65, "Signatures size shortage");

        _hash = keccak256(
            abi.encodePacked(
                "\x19Ethereum Signed Message:\n72",
                _hash,
                chainid,
                expiration
            )
        );
        (address recovered, ) = ECDSA.tryRecover(
            _hash,
            BytesLib.slice(signatures, index, 65)
        );
        return recovered;
    }

    /**
     * Add the address into the signers.
     * @param _address Allowed address.
     */
    function addSigner(
        address _address,
        uint64 expiration,
        bytes memory signatures
    ) external {
        require(_address != address(0), "Signer is zero address.");

        bytes32 _hash = keccak256(
            abi.encodePacked(
                nonce,
                address(this),
                abi.encodeWithSelector(Signers.addSigner.selector, _address)
            )
        );
        require(
            verifySignatures(_hash, expiration, signatures),
            "Invalid signatures"
        );

        require(!_contains(_address), "already added");
        _signers.push(_address);
        _signerId[_address] = _signers.length;

        nonce++;
        emit SignerAdded(_address);
    }

    /**
     * Remove the address from the signers.
     * @param _address Removed address.
     */
    function removeSigner(
        address _address,
        uint64 expiration,
        bytes memory signatures
    ) external {
        bytes32 _hash = keccak256(
            abi.encodePacked(
                nonce,
                address(this),
                abi.encodeWithSelector(Signers.removeSigner.selector, _address)
            )
        );
        require(
            verifySignatures(_hash, expiration, signatures),
            "Invalid signatures"
        );

        require(_contains(_address), "address not found");

        uint256 length = _signers.length;
        require(length - 1 >= threshold, "Signer shortage.");

        uint256 id = _signerId[_address];
        address last = _signers[length - 1];
        _signers[id - 1] = last;
        _signers.pop();
        _signerId[last] = id;
        _signerId[_address] = 0;

        nonce++;
        emit SignerRemoved(_address);
    }

    /**
     * Update the verification threshold.
     * @param _threshold Verification threshold.
     */
    function updateThreshold(
        uint256 _threshold,
        uint64 expiration,
        bytes memory signatures
    ) external {
        require(_threshold > 0, "Threshold is zero.");

        if (threshold == _threshold) {
            return;
        }

        bytes32 _hash = keccak256(
            abi.encodePacked(
                nonce,
                address(this),
                abi.encodeWithSelector(
                    Signers.updateThreshold.selector,
                    _threshold
                )
            )
        );
        require(
            verifySignatures(_hash, expiration, signatures),
            "Invalid signatures"
        );

        require(_signers.length >= _threshold, "Signer shortage.");
        threshold = _threshold;

        nonce++;
        emit ThresholdUpdated(_threshold);
    }

    /**
     * Returns the allowlist.
     */
    function getSigners() external view returns (address[] memory) {
        return _signers;
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
        return _signerId[_address] > 0;
    }
}
