# OpenAPI Thrift TODO

> Active-only tracker. Completed notes belong in `docs/releases/unreleased/` or stable docs.

## Active Work

- [ ] 下一次 npm 发布前把版本从已发布的 `0.1.0` 提升到新的未占用版本，并重新执行完整 prepublish gate；本轮只对齐 canonical 名称，不发布或覆盖既有版本。
- [ ] Run GitHub Actions CI at least once and observe whether `pnpm audit` has environment-specific noise.
- [ ] Configure `NPM_TOKEN` before enabling GitHub Actions automatic npm publish.
