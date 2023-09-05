// SPDX-License-Identifier: MIT
pragma solidity 0.8.12;

interface INFTBridgeSidechain {
    /**********
     * Events *
     **********/

    event SidechainERC721Created(
        uint256 indexed mainchainId,
        address indexed mainchainERC721,
        address sidechainERC721,
        string name,
        string symbol
    );

    event DepositFinalized(
        uint256 indexed mainchainId,
        uint256 indexed depositIndex,
        address mainchainERC721,
        address sidechainERC721,
        uint256 tokenId,
        address mainFrom,
        address sideTo
    );

    event DepositFailed(
        uint256 indexed mainchainId,
        uint256 indexed depositIndex,
        address mainchainERC721,
        address sidechainERC721,
        uint256 tokenId,
        address mainFrom,
        address sideTo
    );

    event WithdrawalInitiated(
        uint256 indexed withdrawalIndex,
        uint256 indexed mainchainId,
        uint256 indexed depositIndex,
        address mainchainERC721,
        address sidechainERC721,
        uint256 tokenId,
        address sideFrom,
        address mainTo
    );

    event WithdrawalRejected(
        uint256 indexed withdrawalIndex,
        uint256 indexed mainchainId,
        uint256 indexed depositIndex
    );

    /***********
     * Structs *
     ***********/

    struct WithdrawalInfo {
        address sidechainERC721;
        uint256 tokenId;
        address sideFrom;
        bool rejected;
    }

    /********************
     * Public Functions *
     ********************/

    function getSidechainERC721(uint256 mainchainId, address mainchainERC721)
        external
        view
        returns (address);

    function getWithdrawalInfo(uint256 withdrawalIndex)
        external
        view
        returns (WithdrawalInfo memory);

    function createSidechainERC721(
        uint256 sidechainId,
        uint256 mainchainId,
        address mainchainERC721,
        string memory name,
        string memory symbol
    ) external;

    function finalizeDeposit(
        uint256 sidechainId,
        uint256 mainchainId,
        uint256 depositIndex,
        address mainchainERC721,
        uint256 tokenId,
        address mainFrom,
        address sideTo
    ) external;

    function withdraw(
        address sidechainERC721,
        uint256 tokenId,
        address mainTo
    ) external;

    function rejectWithdrawal(uint256 sidechainId, uint256 withdrawalIndex)
        external;

    function transferSidechainRelayer(uint256 sidechainId, address newRelayer)
        external;
}
