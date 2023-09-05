// SPDX-License-Identifier: MIT
pragma solidity 0.8.12;

import { INFTBridgeMainchain } from "./INFTBridgeMainchain.sol";
import { INFTBridgeSidechain } from "./INFTBridgeSidechain.sol";
import { Signers } from "./Signers.sol";

contract NFTBridgeRelayer is Signers {
    /**********************
     * Contract Variables *
     **********************/

    address public mainchainBridge;
    address public sidechainBridge;

    /***************
     * Constructor *
     ***************/

    /**
     * @param _mainchainBridge Address of the mainchain bridge.
     * @param _sidechainBridge Address of the sidechain bridge.
     * @param signers Addresses of the signers.
     * @param threshold Threshold of the signatures.
     */
    constructor(
        address _mainchainBridge,
        address _sidechainBridge,
        address[] memory signers,
        uint256 threshold
    ) Signers(signers, threshold) {
        mainchainBridge = _mainchainBridge;
        sidechainBridge = _sidechainBridge;
    }

    /********************
     * Public Functions *
     ********************/

    function rejectDeposit(
        uint256 mainchainId,
        uint256 depositIndex,
        uint64 expiration,
        bytes memory signatures
    ) external {
        bytes32 _hash = keccak256(
            abi.encodeWithSelector(
                INFTBridgeMainchain.rejectDeposit.selector,
                mainchainId,
                depositIndex
            )
        );
        require(
            verifySignatures(_hash, expiration, signatures),
            "Invalid signatures"
        );

        INFTBridgeMainchain(mainchainBridge).rejectDeposit(
            mainchainId,
            depositIndex
        );
    }

    function finalizeWithdrawal(
        uint256 mainchainId,
        uint256 depositIndex,
        uint256 sidechainId,
        uint256 withdrawalIndex,
        address sideFrom,
        address mainTo,
        uint64 expiration,
        bytes memory signatures
    ) external {
        bytes32 _hash = keccak256(
            abi.encodeWithSelector(
                INFTBridgeMainchain.finalizeWithdrawal.selector,
                mainchainId,
                depositIndex,
                sidechainId,
                withdrawalIndex,
                sideFrom,
                mainTo
            )
        );
        require(
            verifySignatures(_hash, expiration, signatures),
            "Invalid signatures"
        );

        INFTBridgeMainchain(mainchainBridge).finalizeWithdrawal(
            mainchainId,
            depositIndex,
            sidechainId,
            withdrawalIndex,
            sideFrom,
            mainTo
        );
    }

    function transferMainchainRelayer(
        uint256 mainchainId,
        address newRelayer,
        uint64 expiration,
        bytes memory signatures
    ) external {
        bytes32 _hash = keccak256(
            abi.encodePacked(
                nonce,
                address(this),
                abi.encodeWithSelector(
                    INFTBridgeMainchain.transferMainchainRelayer.selector,
                    mainchainId,
                    newRelayer
                )
            )
        );
        require(
            verifySignatures(_hash, expiration, signatures),
            "Invalid signatures"
        );

        INFTBridgeMainchain(mainchainBridge).transferMainchainRelayer(
            mainchainId,
            newRelayer
        );

        nonce++;
    }

    function createSidechainERC721(
        uint256 sidechainId,
        uint256 mainchainId,
        address mainERC721,
        string memory name,
        string memory symbol,
        uint64 expiration,
        bytes memory signatures
    ) external {
        bytes32 _hash = keccak256(
            abi.encodeWithSelector(
                INFTBridgeSidechain.createSidechainERC721.selector,
                sidechainId,
                mainchainId,
                mainERC721,
                name,
                symbol
            )
        );
        require(
            verifySignatures(_hash, expiration, signatures),
            "Invalid signatures"
        );

        INFTBridgeSidechain(sidechainBridge).createSidechainERC721(
            sidechainId,
            mainchainId,
            mainERC721,
            name,
            symbol
        );
    }

    function finalizeDeposit(
        uint256 sidechainId,
        uint256 mainchainId,
        uint256 depositIndex,
        address mainERC721,
        uint256 tokenId,
        address mainFrom,
        address sideTo,
        uint64 expiration,
        bytes memory signatures
    ) external {
        bytes32 _hash = keccak256(
            abi.encodeWithSelector(
                INFTBridgeSidechain.finalizeDeposit.selector,
                sidechainId,
                mainchainId,
                depositIndex,
                mainERC721,
                tokenId,
                mainFrom,
                sideTo
            )
        );
        require(
            verifySignatures(_hash, expiration, signatures),
            "Invalid signatures"
        );

        INFTBridgeSidechain(sidechainBridge).finalizeDeposit(
            sidechainId,
            mainchainId,
            depositIndex,
            mainERC721,
            tokenId,
            mainFrom,
            sideTo
        );
    }

    function rejectWithdrawal(
        uint256 sidechainId,
        uint256 withdrawalIndex,
        uint64 expiration,
        bytes memory signatures
    ) external {
        bytes32 _hash = keccak256(
            abi.encodeWithSelector(
                INFTBridgeSidechain.rejectWithdrawal.selector,
                sidechainId,
                withdrawalIndex
            )
        );
        require(
            verifySignatures(_hash, expiration, signatures),
            "Invalid signatures"
        );

        INFTBridgeSidechain(sidechainBridge).rejectWithdrawal(
            sidechainId,
            withdrawalIndex
        );
    }

    function transferSidechainRelayer(
        uint256 sidechainId,
        address newRelayer,
        uint64 expiration,
        bytes memory signatures
    ) external {
        bytes32 _hash = keccak256(
            abi.encodePacked(
                nonce,
                address(this),
                abi.encodeWithSelector(
                    INFTBridgeSidechain.transferSidechainRelayer.selector,
                    sidechainId,
                    newRelayer
                )
            )
        );
        require(
            verifySignatures(_hash, expiration, signatures),
            "Invalid signatures"
        );

        INFTBridgeSidechain(sidechainBridge).transferSidechainRelayer(
            sidechainId,
            newRelayer
        );

        nonce++;
    }
}
