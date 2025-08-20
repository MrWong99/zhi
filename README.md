![zhi](assets/banner.png)

# zhi

**zhi** is a modern platform for **configuration management** and **provisioning**.  
It provides an extensible plugin system that automatically generates a user-friendly terminal UI, while keeping all configurations **secure by default**.

Core principles:

1. **Configuration & Validation** – structured, validated settings that prevent misconfiguration before deployment.
2. **Secure Storage** – encrypted at rest, ensuring confidentiality and compliance without extra setup.
3. **Modularity** – built around an extensible plugin system, enabling support for multiple provisioning backends (Docker Compose, Kubernetes, Helm, and beyond).

---

## Introduction

Managing complex systems requires tools that are both **powerful** and **approachable**.  
**zhi** was designed to bridge the gap between secure configuration handling and intuitive provisioning workflows.  

- **Configuration**: Define, validate, and manage settings consistently.
- **Provisioning**: Deploy to your environment of choice, starting with Docker Compose and Kubernetes/Helm.
- **Extensibility**: Plugins automatically gain a terminal UI, reducing friction for both developers and operators.
---

## Features

- 🔐 **Encrypted configuration at rest**  
- ⚙️ **Automatic validation of settings**  
- 🧩 **Plugin-based extensibility [leveraging gRPC](https://github.com/hashicorp/go-plugin)**  
- 🖥️ **Auto-generated TUI for all modules, with love and [🫧 charm](https://github.com/charmbracelet/bubbletea)**  
- 🚀 **Provisioning with Docker Compose & Kubernetes/Helm (or just bring your own)**  

---

## Getting Started

*Coming soon…*

---

### The Origin of the Name

The name **zhi** (pronounced roughly *“jrr”* in Mandarin) carries layered meanings:

- In **Chinese (置)**, *zhì* means *“to place, set, arrange”* — directly resonating with configuration and provisioning.
- In **Chinese (智)**, *zhì* means *“wisdom, knowledge”* — emphasizing security, clarity, and correctness.

This multiplicity reflects the project’s vision:  
to **arrange systems securely**, with **wisdom in design**, while remaining **flexible and modular**.
