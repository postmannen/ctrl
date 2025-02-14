/* jshint esversion: 9 */
import { createUser, fromPublic, fromSeed } from "https://esm.run/@nats-io/nkeys";
// import { createUser, fromSeed } from "https://cdn.jsdelivr.net/npm/@nats.io/nkeys@3.1.1/lib/nkeys.js";

const hamburgerMenu = document.querySelector(".hamburger-menu");
const generateNkeysBtn = document.getElementById("generateNkeysBtn");
const saveSettingsBtn = document.getElementById("saveSettingsBtn");
const natsServerInput = document.getElementById("natsServer");
const defaultNodeInput = document.getElementById("defaultNode");
const nkeysSeedInput = document.getElementById("nkeysSeed");
const nkeysPublicInput = document.getElementById("nkeysPublic");
const useNkeysCheckbox = document.getElementById("useNkeys");
const clearAllBtn = document.getElementById("clearAllBtn");
const nkeysSeedRawInput = document.getElementById("nkeysSeedRaw");
const updateKeysBtn = document.getElementById("updateKeysBtn");

// Global settings object
const settings = {
    natsServer: "ws://localhost:4223",
    defaultNode: "btdev1",
    nkeysSeed: "",
    nkeysPublic: "",
    useNkeys: false,
    nkeysSeedRAW: null
};

// Add input event listeners to update settings object
natsServerInput.addEventListener('input', () => {
    settings.natsServer = natsServerInput.value;
});

defaultNodeInput.addEventListener('input', () => {
    settings.defaultNode = defaultNodeInput.value;
});

useNkeysCheckbox.addEventListener('change', () => {
    settings.useNkeys = useNkeysCheckbox.checked;
});

hamburgerMenu.addEventListener("click", function () {
    document.body.classList.toggle("menu-open");
});

// Add click handler for the generate nkeys button
generateNkeysBtn.addEventListener("click", function () {
    // create an user nkey KeyPair
    const user = createUser();
    // A seed is the public and private keys together.
    const seed = user.getSeed();

    // console.log("DEBUG: seed array:", Array.from(seed));
    const publicKey = user.getPublicKey();

    // Update settings object
    settings.nkeysSeed = new TextDecoder().decode(seed);
    settings.nkeysPublic = publicKey;
    settings.nkeysSeedRAW = Array.from(seed); // Store as regular array for JSON serialization
    
    // console.log("DEBUG: settings.nkeysSeedRAW:", settings.nkeysSeedRAW);
    
    // Update the input fields
    nkeysSeedInput.value = settings.nkeysSeed;
    nkeysPublicInput.value = settings.nkeysPublic;
    nkeysSeedRawInput.value = settings.nkeysSeedRAW.join(',');
});

// Load existing settings when page loads
document.addEventListener("DOMContentLoaded", function () {
    const savedSettings = localStorage.getItem("settings");
    
    if (savedSettings) {
        // Update our settings object with saved values
        const loadedSettings = JSON.parse(savedSettings);
        Object.assign(settings, loadedSettings);

        // Update UI with values from settings object
        natsServerInput.value = settings.natsServer;
        defaultNodeInput.value = settings.defaultNode;
        nkeysSeedInput.value = settings.nkeysSeed || "";
        nkeysPublicInput.value = settings.nkeysPublic || "";
        nkeysSeedRawInput.value = settings.nkeysSeedRAW ? settings.nkeysSeedRAW.join(',') : "";
        useNkeysCheckbox.checked = settings.useNkeys;
        
        if (settings.nkeysSeedRAW) {
            // Convert the stored array back to Uint8Array
            settings.nkeysSeedRAW = Uint8Array.from(settings.nkeysSeedRAW);
        }
    }

    // console.log("DEBUG: Loaded settings:", settings);
    // console.log("DEBUG: Loaded nkeysSeedRAW:", settings.nkeysSeedRAW ? Array.from(settings.nkeysSeedRAW) : "not set");
});

// Handle save settings button click
saveSettingsBtn.addEventListener("click", function () {
    // Save settings object directly to localStorage
    localStorage.setItem("settings", JSON.stringify(settings));

    // Show save confirmation
    alert("Settings saved successfully!");
});

// Handle clear all button click
clearAllBtn.addEventListener("click", function () {
    // Show confirmation dialog
    if (confirm("Are you sure you want to clear all information? This will delete all settings and file templates.")) {
        // Clear localStorage
        localStorage.clear();
        
        // Reset settings object to defaults
        settings.natsServer = "ws://localhost:4223";
        settings.defaultNode = "btdev1";
        settings.nkeysSeed = "";
        settings.nkeysPublic = "";
        settings.useNkeys = false;
        settings.nkeysSeedRAW = null;
        
        // Reset form fields
        natsServerInput.value = settings.natsServer;
        defaultNodeInput.value = settings.defaultNode;
        nkeysSeedInput.value = "";
        nkeysPublicInput.value = "";
        useNkeysCheckbox.checked = false;
        
        alert("All information has been cleared.");
    }
});

// Add click handler for update keys button
updateKeysBtn.addEventListener('click', function() {
    try {
        // Convert input string to array of numbers
        const rawArray = nkeysSeedRawInput.value.split(',').map(num => parseInt(num.trim()));
        const seedRaw = new Uint8Array(rawArray);
        
        // Create key pair from seed
        const keyPair = fromSeed(seedRaw);
        
        // Update settings and fields
        settings.nkeysSeedRAW = Array.from(seedRaw);
        settings.nkeysSeed = new TextDecoder().decode(seedRaw);
        settings.nkeysPublic = keyPair.getPublicKey();
        
        nkeysSeedInput.value = settings.nkeysSeed;
        nkeysPublicInput.value = settings.nkeysPublic;
    } catch (error) {
        console.error("Invalid seed raw value:", error);
        alert("Invalid seed raw value. Please check the format.");
    }
}); 