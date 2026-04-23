// SPDX-License-Identifier: MIT
pragma solidity 0.8.24;

import { IERC20 } from "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import { IERC20Metadata } from "@openzeppelin/contracts/token/ERC20/extensions/IERC20Metadata.sol";
import { IERC20Permit } from "@openzeppelin/contracts/token/ERC20/extensions/IERC20Permit.sol";

/// @title IGoldToken
/// @notice 1 GOLD = 1 gram fiziksel altın. 18 decimals. ERC-20 + permit + pausable.
/// @dev Mint/burn sadece yetkili controller adresleri üzerinden. Tüm transferler
///      ComplianceRegistry.canTransfer() kapısından geçer.
interface IGoldToken is IERC20, IERC20Metadata, IERC20Permit {
    /// @notice Aktif compliance registry adresi.
    function complianceRegistry() external view returns (address);

    /// @notice Yetkili mint controller — yalnızca bu adres mint() çağırabilir.
    function mintController() external view returns (address);

    /// @notice Yetkili burn controller — yalnızca bu adres burnFrom() çağırabilir.
    function burnController() external view returns (address);

    /// @notice Controller tarafından token basımı.
    /// @dev _update içinde compliance kontrolü mint için ayrı ele alınır (canMint).
    function mint(address to, uint256 amount, bytes2 jurisdiction) external;

    /// @notice Controller tarafından token yakımı (kullanıcı adına).
    /// @dev Kullanıcı izni approve ile verilmelidir; controller pull-burn yapar.
    function burnFrom(address from, uint256 amount) external;

    /// @notice Acil pause (sadece PAUSER_ROLE).
    function pause() external;

    /// @notice Unpause (sadece Treasury).
    function unpause() external;

    /// @notice Pause durumu.
    function paused() external view returns (bool);

    // Yönetim
    function setComplianceRegistry(address newRegistry) external;
    function setMintController(address newController) external;
    function setBurnController(address newController) external;

    event ComplianceRegistryUpdated(address indexed oldAddr, address indexed newAddr);
    event MintControllerUpdated(address indexed oldAddr, address indexed newAddr);
    event BurnControllerUpdated(address indexed oldAddr, address indexed newAddr);
    event Minted(address indexed to, uint256 amount, bytes2 jurisdiction);
    event Burned(address indexed from, uint256 amount);
}
