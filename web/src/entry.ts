// Entry point. The zod config has to run before any module that builds a schema, and import order is
// only guaranteed to be kept between separate modules, not inside main.tsx (the import sorter moves it).
import './lib/zod-config'
import './main'
