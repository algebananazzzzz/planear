# Planear vs. Other Tools

Planear is designed for declarative data and infrastructure reconciliation with complete customizability. Here's how it compares to similar tools.

## Planear vs. Terraform

**Terraform** is excellent for cloud infrastructure, but it has fundamental limitations when it comes to custom data reconciliation:

| Aspect | Planear | Terraform | Notes |
|--------|---------|-----------|-------|
| **Customization** | Full Go control | Limited by provider ecosystem | Even with custom providers, you're writing HCL + provider plugins |
| **Development Speed** | Hours (pure Go) | Days (HCL + provider development) | Planear: write callbacks in Go. Terraform: design provider interface, plugin SDK, state handling |
| **Type Safety** | ✅ Full generics | ❌ HCL dynamic typing | Planear catches type errors at compile time |
| **Data Types** | Any Go struct | Predefined schema | Planear: use any type. Terraform: limited to string/number/bool/list/map |
| **Debugging** | ✅ Native Go debugger | ⚠️ Unclear HCL execution | Step through your exact logic with standard Go tools |
| **Dependency Management** | Go modules | Provider version matrix | Terraform version + provider version + state compatibility |
| **Learning Curve** | Minimal (if you know Go) | Steep (HCL + state concepts) | Planear is just Go. Terraform requires learning provider model |
| **Integration** | Direct API/DB calls | Must write provider | Need custom authentication? Direct call your auth system in Go |
| **Vendor Lock-in** | ❌ None (pure Go) | ⚠️ Partial (Terraform state) | Planear: export at any time. Terraform: state file portability issues |

**Even with custom Terraform providers:** Creating a custom provider takes 10x longer than writing Planear callbacks, requires maintaining provider SDK compatibility, and still forces you into Terraform's state management model.

## Planear vs. Pulumi

**Pulumi** is closer to Planear (both use programming languages), but differs significantly:

| Aspect | Planear | Pulumi | Notes |
|--------|---------|--------|-------|
| **Purpose** | Data/resource reconciliation | Infrastructure as Code | Different use cases entirely |
| **Declarative CSV** | ✅ Built-in | ❌ Not designed for this | Pulumi: define infra in code. Planear: sync CSV to any system |
| **Plan Before Execute** | ✅ Always (diff-based) | ⚠️ Through Pulumi Automation | Planear generates visible diffs. Pulumi requires Automation API |
| **Custom Logic** | Simple callbacks | Complex stack model | Planear: `OnAdd`, `OnUpdate`, etc. Pulumi: full stack lifecycle |
| **Learning Curve** | Minimal | Moderate (stack concepts) | Planear: just Go functions. Pulumi: resource model, stacks, state |
| **Execution Speed** | Fast (simple callbacks) | Slower (full stack evaluation) | Planear optimized for data ops. Pulumi optimized for IaC |
| **CSV-Driven Workflows** | ✅ Perfect fit | ⚠️ Awkward workaround | Planear designed for declarative CSV state |

## Planear vs. Liquibase / Flyway

**Liquibase** is a database-specific migration tool; Planear is general-purpose data reconciliation:

| Aspect | Planear | Liquibase | Notes |
|--------|---------|-----------|-------|
| **Scope** | General data reconciliation | Database schema migrations | Different problem domains |
| **Customization** | Full Go control | XML/YAML + limited plugins | Liquibase is prescriptive, Planear is flexible |
| **CSV Support** | ✅ First-class | ❌ Not designed for this | Planear: CSV = source of truth. Liquibase: SQL changes |
| **Any Data Source** | ✅ (custom callbacks) | ❌ (database-focused) | Need to sync APIs, external systems? Planear wins |
| **Rollback Capability** | ❌ Manual (by design) | ✅ Built-in | Liquibase: designed for migrations. Planear: explicit operations |
| **Idempotency** | ✅ CSV re-run safe | Varies by changeset | Planear: always safe to re-run |

**When to use Liquibase:** Tracking database schema versions with automatic rollback
**When to use Planear:** Keeping CSV definitions in sync with users, configs, permissions, or any custom data system

## Planear vs. Custom Scripts

Writing custom reconciliation logic from scratch is common but leaves you reinventing:

| Aspect | Planear | Custom Scripts |
|--------|---------|-----------------|
| **Declarative State** | ✅ CSV-driven | ❌ Imperative |
| **Plan Before Execute** | ✅ Visible diffs | ❌ Runs immediately |
| **Error Recovery** | ✅ Retry logic | ❌ Manual handling |
| **Parallelization** | ✅ Built-in worker pool | ❌ Manual threading |
| **Field-Level Diffs** | ✅ Automatic tracking | ❌ Manual comparison |
| **Development Time** | Medium | Long (everything from scratch) |
| **Maintenance** | Low (library handles boilerplate) | High (you own everything) |

## Summary

- **Use Terraform/Pulumi**: Cloud infrastructure management with defined provider ecosystem
- **Use Liquibase**: Database schema versioning with rollback capabilities
- **Use Planear**: Declarative data reconciliation with complete Go customization
- **Use Custom Scripts**: Simple one-off operations (but consider Planear for anything recurring)
