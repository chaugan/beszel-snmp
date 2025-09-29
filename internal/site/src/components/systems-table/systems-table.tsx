import {
	ColumnDef,
	ColumnFiltersState,
	getFilteredRowModel,
	SortingState,
	getSortedRowModel,
	flexRender,
	VisibilityState,
	getCoreRowModel,
	useReactTable,
	Row,
	Table as TableType,
} from "@tanstack/react-table"
import { TableBody, TableCell, TableHead, TableHeader, TableRow } from "@/components/ui/table"
import { Button } from "@/components/ui/button"
import {
	DropdownMenu,
	DropdownMenuCheckboxItem,
	DropdownMenuContent,
	DropdownMenuItem,
	DropdownMenuLabel,
	DropdownMenuRadioGroup,
	DropdownMenuRadioItem,
	DropdownMenuSeparator,
	DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { SystemRecord } from "@/types"
import {
	ArrowUpDownIcon,
	LayoutGridIcon,
	LayoutListIcon,
	ArrowDownIcon,
	ArrowUpIcon,
	Settings2Icon,
	EyeIcon,
	FilterIcon,
	ThermometerIcon,
	Droplets,
	AudioWaveform,
	Gauge,
	Cloud,
	Wind,
} from "lucide-react"
import { memo, useEffect, useMemo, useRef, useState } from "react"
import { $pausedSystems, $downSystems, $upSystems, $systems, $userSettings } from "@/lib/stores"
import { useStore } from "@nanostores/react"
import { cn, runOnce, useBrowserStorage, decimalString, formatTemperature } from "@/lib/utils"
import { $router, Link } from "../router"
import { useLingui, Trans } from "@lingui/react/macro"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "../ui/card"
import { Input } from "@/components/ui/input"
import { getPagePath } from "@nanostores/router"
import SystemsTableColumns, { ActionsButton, IndicatorDot, sortableHeader } from "./systems-table-columns"
import AlertButton from "../alerts/alert-button"
import { SystemStatus } from "@/lib/enums"
import { useVirtualizer, VirtualItem } from "@tanstack/react-virtual"
import { pb } from "@/lib/api"
import type { SystemStatsRecord } from "@/types"

// Optimized hook to fetch latest stats data for SNMP agents using batch queries and real-time subscriptions
function useSnmpStats(snmpData: SystemRecord[]) {
	const [snmpStats, setSnmpStats] = useState<Record<string, SystemStatsRecord | null>>({})
	
	useEffect(() => {
		// Only fetch if we have SNMP data
		if (snmpData.length === 0) {
			return
		}
		
		const systemIds = snmpData.map(system => system.id)
		let unsubscribe: (() => void) | undefined
		
		const fetchStats = async () => {
			try {
				// Use a single optimized query to get latest stats for all SNMP systems
				// Build filter for multiple system IDs
				const systemIdFilter = systemIds.map(id => `system='${id}'`).join(' || ')
				const filter = `(${systemIdFilter}) && type='1m'`
				
				const stats = await pb.collection<SystemStatsRecord>("system_stats").getFullList({
					filter: filter,
					sort: "-created",
					fields: "id,created,system,stats,type",
				})
				
				// Group stats by system ID and take only the latest for each system
				const statsMap: Record<string, SystemStatsRecord | null> = {}
				const latestBySystem: Record<string, SystemStatsRecord> = {}
				
				// Since results are sorted by -created, first occurrence of each system is the latest
				for (const stat of stats) {
					if (!(stat.system in latestBySystem)) {
						latestBySystem[stat.system] = stat
					}
				}
				
				// Create final map ensuring all systems have an entry
				systemIds.forEach(systemId => {
					statsMap[systemId] = latestBySystem[systemId] || null
				})
				
				setSnmpStats(statsMap)
			} catch (error) {
				console.error("Failed to fetch SNMP stats:", error)
				// Set all to null on error
				const errorMap: Record<string, SystemStatsRecord | null> = {}
				systemIds.forEach(systemId => {
					errorMap[systemId] = null
				})
				setSnmpStats(errorMap)
			}
		}
		
		// Subscribe to real-time updates for system_stats collection
		const subscribeToUpdates = async () => {
			try {
				unsubscribe = await pb.collection<SystemStatsRecord>("system_stats").subscribe(
					"*",
					({ action, record }) => {
						// Only process records for our SNMP systems
						if (!systemIds.includes(record.system)) {
							return
						}
						
						setSnmpStats(prevStats => {
							const newStats = { ...prevStats }
							
							if (action === "create" || action === "update") {
								// Update with the latest record for this system
								newStats[record.system] = record
							} else if (action === "delete") {
								// Set to null if record is deleted
								newStats[record.system] = null
							}
							
							return newStats
						})
					},
					{
						filter: `(${systemIds.map(id => `system='${id}'`).join(' || ')}) && type='1m'`,
						fields: "id,created,system,stats,type",
					}
				)
			} catch (error) {
				console.error("Failed to subscribe to SNMP stats updates:", error)
			}
		}
		
		// Initial fetch
		fetchStats()
		
		// Subscribe to real-time updates
		subscribeToUpdates()
		
		// Cleanup subscription on unmount
		return () => {
			unsubscribe?.()
		}
	}, [snmpData])
	
	return snmpStats
}


type ViewMode = "table" | "grid"
type StatusFilter = "all" | SystemRecord["status"]

const preloadSystemDetail = runOnce(() => import("@/components/routes/system.tsx"))

// Build SNMP-specific columns for the secondary table
function SystemsSnmpTableColumns(snmpStats: Record<string, SystemStatsRecord | null>): ColumnDef<SystemRecord>[] {
	return [
		{
			accessorKey: "name",
			id: "system",
			header: () => <span className="px-1.5">System</span>,
			cell: (info) => {
				const { name } = info.row.original
				return (
					<>
						<span className="flex gap-2 items-center font-medium text-sm text-nowrap md:ps-1">
							<IndicatorDot system={info.row.original} />
							<span className="truncate max-w-48">{name}</span>
						</span>
						<Link
							href={getPagePath($router, "system", { name })}
							className="inset-0 absolute size-full"
							aria-label={name}
						></Link>
					</>
				)
			},
		},
		{
			accessorKey: "id",
			id: "temp",
			name: () => "Temp",
			Icon: ThermometerIcon,
			header: () => <span className="px-1.5">Temp</span>,
			cell(info) {
				const system = info.row.original
				const stats = snmpStats[system.id]
				const userSettings = useStore($userSettings, { keys: ["unitTemp"] })
				
				// Get temperature from stats data
				const tempSensors = stats?.stats?.t
				if (!tempSensors || Object.keys(tempSensors).length === 0) {
					return null
				}
				
				// Take the first temperature reading
				const firstSensorKey = Object.keys(tempSensors)[0]
				const val = tempSensors[firstSensorKey]
				
				const { value, unit } = formatTemperature(val, userSettings.unitTemp)
				return (
					<span className="tabular-nums whitespace-nowrap">
						{decimalString(value, value >= 100 ? 1 : 2)} {unit}
					</span>
				)
			},
		},
		{
			accessorKey: "id",
			id: "humidity",
			name: () => "Humidity",
			Icon: Droplets,
			header: () => <span className="px-1.5">Humidity</span>,
			cell(info) {
				const system = info.row.original
				const stats = snmpStats[system.id]
				
				// Get humidity from stats data
				const humidSensors = stats?.stats?.h
				if (!humidSensors || Object.keys(humidSensors).length === 0) {
					return null
				}
				
				// Take the first humidity reading
				const firstSensorKey = Object.keys(humidSensors)[0]
				const v = humidSensors[firstSensorKey]
				
				return <span className="tabular-nums whitespace-nowrap">{decimalString(Number(v), Number(v) >= 100 ? 0 : 1)} %</span>
			},
		},
		{
			accessorKey: "id",
			id: "co2",
			name: () => "CO2",
			Icon: AudioWaveform,
			header: () => <span className="px-1.5">CO2</span>,
			cell(info) {
				const system = info.row.original
				const stats = snmpStats[system.id]
				
				// Get CO2 from stats data
				const co2Sensors = stats?.stats?.co2
				if (!co2Sensors || Object.keys(co2Sensors).length === 0) {
					return null
				}
				
				// Take the first CO2 reading
				const firstSensorKey = Object.keys(co2Sensors)[0]
				const v = co2Sensors[firstSensorKey]
				
				return <span className="tabular-nums whitespace-nowrap">{decimalString(Number(v), Number(v) >= 1000 ? 0 : 1)} ppm</span>
			},
		},
		{
			accessorKey: "id",
			id: "pressure",
			name: () => "Pressure",
			Icon: Gauge,
			header: () => <span className="px-1.5">Pressure</span>,
			cell(info) {
				const system = info.row.original
				const stats = snmpStats[system.id]
				
				// Get pressure from stats data
				const pressureSensors = stats?.stats?.pr
				if (!pressureSensors || Object.keys(pressureSensors).length === 0) {
					return null
				}
				
				// Take the first pressure reading
				const firstSensorKey = Object.keys(pressureSensors)[0]
				const v = pressureSensors[firstSensorKey]
				
				return <span className="tabular-nums whitespace-nowrap">{decimalString(Number(v), Number(v) >= 100 ? 0 : 1)} hPa</span>
			},
		},
		{
			accessorKey: "id",
			id: "pm25",
			name: () => "PM2.5",
			Icon: Cloud,
			header: () => <span className="px-1.5">PM2.5</span>,
			cell(info) {
				const system = info.row.original
				const stats = snmpStats[system.id]
				
				// Get PM2.5 from stats data
				const pm25Sensors = stats?.stats?.pm25
				if (!pm25Sensors || Object.keys(pm25Sensors).length === 0) {
					return null
				}
				
				// Take the first PM2.5 reading
				const firstSensorKey = Object.keys(pm25Sensors)[0]
				const v = pm25Sensors[firstSensorKey]
				
				return <span className="tabular-nums whitespace-nowrap">{decimalString(Number(v), Number(v) >= 100 ? 0 : 1)} µg/m³</span>
			},
		},
		{
			accessorKey: "id",
			id: "pm10",
			name: () => "PM10",
			Icon: Cloud,
			header: () => <span className="px-1.5">PM10</span>,
			cell(info) {
				const system = info.row.original
				const stats = snmpStats[system.id]
				
				// Get PM10 from stats data
				const pm10Sensors = stats?.stats?.pm10
				if (!pm10Sensors || Object.keys(pm10Sensors).length === 0) {
					return null
				}
				
				// Take the first PM10 reading
				const firstSensorKey = Object.keys(pm10Sensors)[0]
				const v = pm10Sensors[firstSensorKey]
				
				return <span className="tabular-nums whitespace-nowrap">{decimalString(Number(v), Number(v) >= 100 ? 0 : 1)} µg/m³</span>
			},
		},
		{
			accessorKey: "id",
			id: "voc",
			name: () => "VOC",
			Icon: Wind,
			header: () => <span className="px-1.5">VOC</span>,
			cell(info) {
				const system = info.row.original
				const stats = snmpStats[system.id]
				
				// Get VOC from stats data
				const vocSensors = stats?.stats?.voc
				if (!vocSensors || Object.keys(vocSensors).length === 0) {
					return null
				}
				
				// Take the first VOC reading
				const firstSensorKey = Object.keys(vocSensors)[0]
				const v = vocSensors[firstSensorKey]
				
				return <span className="tabular-nums whitespace-nowrap">{decimalString(Number(v), Number(v) >= 1000 ? 0 : 1)} ppb</span>
			},
		},
		{
			id: "actions",
			header: () => <span className="px-1.5">Actions</span>,
			cell: ({ row }) => (
				<div className="relative z-10 flex justify-end items-center gap-1 -ms-3">
					<AlertButton system={row.original} />
					<ActionsButton system={row.original} />
				</div>
			),
		},
	]
}

const SystemCard = memo(
	({ row, table, colLength }: { row: Row<SystemRecord>; table: TableType<SystemRecord>; colLength: number }) => {
		const system = row.original
		const { t } = useLingui()

		return useMemo(() => {
			return (
				<Card
					onMouseEnter={preloadSystemDetail}
					key={system.id}
					className={cn(
						"cursor-pointer hover:shadow-md transition-all bg-transparent w-full dark:border-border duration-200 relative",
						{
							"opacity-50": system.status === SystemStatus.Paused,
						}
					)}
				>
					<CardHeader className="py-1 ps-5 pe-3 bg-muted/30 border-b border-border/60">
						<div className="flex items-center gap-2 w-full overflow-hidden">
							<CardTitle className="text-base tracking-normal text-primary/90 flex items-center min-w-0 flex-1 gap-2.5">
								<div className="flex items-center gap-2.5 min-w-0 flex-1">
									<IndicatorDot system={system} />
									<span className="text-[.95em]/normal tracking-normal text-primary/90 truncate">{system.name}</span>
								</div>
							</CardTitle>
							{table.getColumn("actions")?.getIsVisible() && (
								<div className="flex gap-1 shrink-0 relative z-10">
									<AlertButton system={system} />
									<ActionsButton system={system} />
								</div>
							)}
						</div>
					</CardHeader>
					<CardContent className="text-sm px-5 pt-3.5 pb-4">
						<div className="grid gap-2.5" style={{ gridTemplateColumns: "24px minmax(80px, max-content) 1fr" }}>
							{table.getAllColumns().map((column) => {
								if (!column.getIsVisible() || column.id === "system" || column.id === "actions") return null
								const cell = row.getAllCells().find((cell) => cell.column.id === column.id)
								if (!cell) return null
								
								// Handle both regular system columns (with Icon/name) and SNMP columns (with header only)
								// @ts-ignore
								const columnDef = column.columnDef as ColumnDef<SystemRecord, unknown>
								const hasIconAndName = 'Icon' in columnDef && 'name' in columnDef
								
								return (
									<>
										<div key={`${column.id}-icon`} className="flex items-center">
											{column.id === "lastSeen" ? (
												<EyeIcon className="size-4 text-muted-foreground" />
											) : hasIconAndName ? (
												// @ts-ignore
												columnDef.Icon && <columnDef.Icon className="size-4 text-muted-foreground" />
											) : (
												<div className="size-4" /> // Empty space for SNMP columns without icons
											)}
										</div>
										<div key={`${column.id}-label`} className="flex items-center text-muted-foreground pr-3">
											{hasIconAndName ? (
												// @ts-ignore
												columnDef.name() + ":"
											) : (
												// For SNMP columns, extract text from header function
												columnDef.header ? 
													(columnDef.header as any)()?.props?.children + ":" : 
													column.id + ":"
											)}
										</div>
										<div key={`${column.id}-value`} className="flex items-center">
											{flexRender(cell.column.columnDef.cell, cell.getContext())}
										</div>
									</>
								)
							})}
						</div>
					</CardContent>
					<Link
						href={getPagePath($router, "system", { name: row.original.name })}
						className="inset-0 absolute w-full h-full"
					>
						<span className="sr-only">{row.original.name}</span>
					</Link>
				</Card>
			)
		}, [system, colLength, t])
	}
)

export default function SystemsTable() {
	const data = useStore($systems)
	const downSystems = $downSystems.get()
	const upSystems = $upSystems.get()
	const pausedSystems = $pausedSystems.get()
	const { i18n, t } = useLingui()
	const [filter, setFilter] = useState<string>()
	const [statusFilter, setStatusFilter] = useState<StatusFilter>("all")
	const [sorting, setSorting] = useBrowserStorage<SortingState>(
		"sortMode",
		[{ id: "system", desc: false }],
		sessionStorage
	)
	const [columnFilters, setColumnFilters] = useState<ColumnFiltersState>([])
	const [columnVisibility, setColumnVisibility] = useBrowserStorage<VisibilityState>("cols", {})

	const locale = i18n.locale

	// Filter data based on status filter
	const filteredData = useMemo(() => {
		if (statusFilter === "all") {
			return data
		}
		if (statusFilter === SystemStatus.Up) {
			return Object.values(upSystems) ?? []
		}
		if (statusFilter === SystemStatus.Down) {
			return Object.values(downSystems) ?? []
		}
		return Object.values(pausedSystems) ?? []
	}, [data, statusFilter])

	// Split into main agents and SNMP agents based on agent type
	const { mainData, snmpData } = useMemo(() => {
		const snmp = filteredData.filter((s) => s?.info?.a === "snmp")
		const main = filteredData.filter((s) => s?.info?.a !== "snmp")
		return { mainData: main, snmpData: snmp }
	}, [filteredData])

	const [viewMode, setViewMode] = useBrowserStorage<ViewMode>(
		"viewMode",
		// show grid view on mobile if there are less than 200 systems (looks better but table is more efficient)
		window.innerWidth < 1024 && filteredData.length < 200 ? "grid" : "table"
	)

	const columnDefs = useMemo(() => SystemsTableColumns(viewMode), [viewMode])
	
	// Fetch stats data for SNMP agents
	const snmpStats = useSnmpStats(snmpData)
	const snmpColumnDefs = useMemo(() => SystemsSnmpTableColumns(snmpStats), [snmpStats])

	const table = useReactTable({
		data: mainData,
		columns: columnDefs,
		getCoreRowModel: getCoreRowModel(),
		onSortingChange: setSorting,
		getSortedRowModel: getSortedRowModel(),
		onColumnFiltersChange: setColumnFilters,
		getFilteredRowModel: getFilteredRowModel(),
		onColumnVisibilityChange: setColumnVisibility,
		state: {
			sorting,
			columnFilters,
			columnVisibility,
		},
		defaultColumn: {
			invertSorting: true,
			sortUndefined: "last",
			minSize: 0,
			size: 900,
			maxSize: 900,
		},
	})

	// Independent table for SNMP systems
	const [snmpSorting, setSnmpSorting] = useState<SortingState>([{ id: "system", desc: false }])
	const [snmpFilters, setSnmpFilters] = useState<ColumnFiltersState>([])
	const [snmpVisibility, setSnmpVisibility] = useState<VisibilityState>({})

	const snmpTable = useReactTable({
		data: snmpData,
		columns: snmpColumnDefs,
		getCoreRowModel: getCoreRowModel(),
		onSortingChange: setSnmpSorting,
		getSortedRowModel: getSortedRowModel(),
		onColumnFiltersChange: setSnmpFilters,
		getFilteredRowModel: getFilteredRowModel(),
		onColumnVisibilityChange: setSnmpVisibility,
		state: {
			sorting: snmpSorting,
			columnFilters: snmpFilters,
			columnVisibility: snmpVisibility,
		},
		defaultColumn: {
			invertSorting: true,
			sortUndefined: "last",
			minSize: 0,
			size: 900,
			maxSize: 900,
		},
	})

	useEffect(() => {
		if (filter !== undefined) {
			table.getColumn("system")?.setFilterValue(filter)
			snmpTable.getColumn("system")?.setFilterValue(filter)
		}
	}, [filter])

	const rows = table.getRowModel().rows
	const columns = table.getAllColumns()
	const visibleColumns = table.getVisibleLeafColumns()

	const snmpRows = snmpTable.getRowModel().rows
	const snmpVisibleColumns = snmpTable.getVisibleLeafColumns()

	const [upSystemsLength, downSystemsLength, pausedSystemsLength] = useMemo(() => {
		return [Object.values(upSystems).length, Object.values(downSystems).length, Object.values(pausedSystems).length]
	}, [upSystems, downSystems, pausedSystems])

	// TODO: hiding temp then gpu messes up table headers
	const CardHead = useMemo(() => {
		return (
			<CardHeader className="pb-4.5 px-2 sm:px-6 max-sm:pt-5 max-sm:pb-1">
				<div className="grid md:flex gap-5 w-full items-end">
					<div className="px-2 sm:px-1">
						<CardTitle className="mb-2">
							<Trans>All Systems</Trans>
						</CardTitle>
						<CardDescription className="flex">
							<Trans>Click on a system to view more information.</Trans>
						</CardDescription>
					</div>

					<div className="flex gap-2 ms-auto w-full md:w-80">
						<Input placeholder={t`Filter...`} onChange={(e) => setFilter(e.target.value)} className="px-4" />
						<DropdownMenu>
							<DropdownMenuTrigger asChild>
								<Button variant="outline">
									<Settings2Icon className="me-1.5 size-4 opacity-80" />
									<Trans>View</Trans>
								</Button>
							</DropdownMenuTrigger>
							<DropdownMenuContent align="end" className="h-72 md:h-auto min-w-48 md:min-w-auto overflow-y-auto">
								<div className="grid grid-cols-1 md:grid-cols-4 divide-y md:divide-s md:divide-y-0">
									<div className="border-r">
										<DropdownMenuLabel className="pt-2 px-3.5 flex items-center gap-2">
											<LayoutGridIcon className="size-4" />
											<Trans>Layout</Trans>
										</DropdownMenuLabel>
										<DropdownMenuSeparator />
										<DropdownMenuRadioGroup
											className="px-1 pb-1"
											value={viewMode}
											onValueChange={(view) => setViewMode(view as ViewMode)}
										>
											<DropdownMenuRadioItem value="table" onSelect={(e) => e.preventDefault()} className="gap-2">
												<LayoutListIcon className="size-4" />
												<Trans>Table</Trans>
											</DropdownMenuRadioItem>
											<DropdownMenuRadioItem value="grid" onSelect={(e) => e.preventDefault()} className="gap-2">
												<LayoutGridIcon className="size-4" />
												<Trans>Grid</Trans>
											</DropdownMenuRadioItem>
										</DropdownMenuRadioGroup>
									</div>

									<div className="border-r">
										<DropdownMenuLabel className="pt-2 px-3.5 flex items-center gap-2">
											<FilterIcon className="size-4" />
											<Trans>Status</Trans>
										</DropdownMenuLabel>
										<DropdownMenuSeparator />
										<DropdownMenuRadioGroup
											className="px-1 pb-1"
											value={statusFilter}
											onValueChange={(value) => setStatusFilter(value as StatusFilter)}
										>
											<DropdownMenuRadioItem value="all" onSelect={(e) => e.preventDefault()}>
												<Trans>All Systems</Trans>
											</DropdownMenuRadioItem>
											<DropdownMenuRadioItem value="up" onSelect={(e) => e.preventDefault()}>
												<Trans>Up ({upSystemsLength})</Trans>
											</DropdownMenuRadioItem>
											<DropdownMenuRadioItem value="down" onSelect={(e) => e.preventDefault()}>
												<Trans>Down ({downSystemsLength})</Trans>
											</DropdownMenuRadioItem>
											<DropdownMenuRadioItem value="paused" onSelect={(e) => e.preventDefault()}>
												<Trans>Paused ({pausedSystemsLength})</Trans>
											</DropdownMenuRadioItem>
										</DropdownMenuRadioGroup>
									</div>

									<div className="border-r">
										<DropdownMenuLabel className="pt-2 px-3.5 flex items-center gap-2">
											<ArrowUpDownIcon className="size-4" />
											<Trans>Sort By</Trans>
										</DropdownMenuLabel>
										<DropdownMenuSeparator />
										<div className="px-1 pb-1">
											{columns.map((column) => {
												if (!column.getCanSort()) return null
												let Icon = <span className="w-6"></span>
												// if current sort column, show sort direction
												if (sorting[0]?.id === column.id) {
													if (sorting[0]?.desc) {
														Icon = <ArrowUpIcon className="me-2 size-4" />
													} else {
														Icon = <ArrowDownIcon className="me-2 size-4" />
													}
												}
												return (
													<DropdownMenuItem
														onSelect={(e) => {
															e.preventDefault()
															setSorting([{ id: column.id, desc: sorting[0]?.id === column.id && !sorting[0]?.desc }])
														}}
														key={column.id}
													>
													{Icon}
													{/* @ts-ignore */}
													{column.columnDef.name()}
												</DropdownMenuItem>
												)
											})}
										</div>
									</div>

									<div>
										<DropdownMenuLabel className="pt-2 px-3.5 flex items-center gap-2">
											<EyeIcon className="size-4" />
											<Trans>Visible Fields</Trans>
										</DropdownMenuLabel>
										<DropdownMenuSeparator />
										<div className="px-1.5 pb-1">
											{columns
													.filter((column) => column.getCanHide())
													.map((column) => {
														return (
															<DropdownMenuCheckboxItem
																key={column.id}
																onSelect={(e) => e.preventDefault()}
																checked={column.getIsVisible()}
																onCheckedChange={(value) => column.toggleVisibility(!!value)}
															>
																{/* @ts-ignore */}
																{column.columnDef.name()}
															</DropdownMenuCheckboxItem>
														)
													})}
										</div>
									</div>
								</div>
							</DropdownMenuContent>
						</DropdownMenu>
					</div>
				</div>
			</CardHeader>
		)
	}, [
		visibleColumns.length,
		sorting,
		viewMode,
		locale,
		statusFilter,
		upSystemsLength,
		downSystemsLength,
		pausedSystemsLength,
	])

	return (
		<Card>
			{CardHead}
			<div className="p-6 pt-0 max-sm:py-3 max-sm:px-2">
				{viewMode === "table" ? (
					// table layout
					<div className="rounded-md space-y-6">
						<AllSystemsTable table={table} rows={rows} colLength={visibleColumns.length} />
						{snmpRows.length > 0 && (
							<div className="space-y-2">
								<div className="text-sm text-muted-foreground px-1.5">
									<Trans>SNMP Sensors</Trans>
								</div>
								<AllSystemsTable table={snmpTable} rows={snmpRows} colLength={snmpVisibleColumns.length} />
							</div>
						)}
					</div>
				) : (
					// grid layout
					<div className="grid gap-4 grid-cols-1 sm:grid-cols-2 lg:grid-cols-3">
						{rows?.length ? (
							rows.map((row) => {
								return <SystemCard key={row.original.id} row={row} table={table} colLength={visibleColumns.length} />
							})
						) : (
							<div className="col-span-full text-center py-8">
								<Trans>No systems found.</Trans>
							</div>
						)}
						
						{/* SNMP systems in grid */}
						{snmpRows?.length > 0 && snmpRows.map((row) => {
							return <SystemCard key={row.original.id} row={row} table={snmpTable} colLength={snmpVisibleColumns.length} />
						})}
					</div>
				)}
			</div>
		</Card>
	)
}

const AllSystemsTable = memo(function ({
	table,
	rows,
	colLength,
}: {
	table: TableType<SystemRecord>
	rows: Row<SystemRecord>[]
	colLength: number
}) {
	// The virtualizer will need a reference to the scrollable container element
	const scrollRef = useRef<HTMLDivElement>(null)

	const virtualizer = useVirtualizer<HTMLDivElement, HTMLTableRowElement>({
		count: rows.length,
		estimateSize: () => (rows.length > 10 ? 56 : 60),
		getScrollElement: () => scrollRef.current,
		overscan: 5,
	})
	const virtualRows = virtualizer.getVirtualItems()

	const paddingTop = Math.max(0, virtualRows[0]?.start ?? 0 - virtualizer.options.scrollMargin)
	const paddingBottom = Math.max(0, virtualizer.getTotalSize() - (virtualRows[virtualRows.length - 1]?.end ?? 0))

	return (
		<div
			className={cn(
				"h-min max-h-[calc(100dvh-17rem)] max-w-full relative overflow-auto border rounded-md",
				// don't set min height if there are less than 2 rows, do set if we need to display the empty state
				(!rows.length || rows.length > 2) && "min-h-50"
			)}
			ref={scrollRef}
		>
			{/* add header height to table size */}
			<div style={{ height: `${virtualizer.getTotalSize() + 50}px`, paddingTop, paddingBottom }}>
				<table className="text-sm w-full h-full">
					<SystemsTableHead table={table} colLength={colLength} />
					<TableBody onMouseEnter={preloadSystemDetail}>
						{rows.length ? (
							virtualRows.map((virtualRow) => {
								const row = rows[virtualRow.index] as Row<SystemRecord>
								return (
									<SystemTableRow
										key={row.id}
										row={row}
										virtualRow={virtualRow}
										length={rows.length}
										colLength={colLength}
									/>
								)
							})
						) : (
							<TableRow>
								<TableCell colSpan={colLength} className="h-37 text-center pointer-events-none">
									<Trans>No systems found.</Trans>
								</TableCell>
							</TableRow>
						)}
					</TableBody>
				</table>
			</div>
		</div>
	)
})

function SystemsTableHead({ table, colLength }: { table: TableType<SystemRecord>; colLength: number }) {
	const { i18n } = useLingui()

	return useMemo(() => {
		return (
			<TableHeader className="sticky top-0 z-20 w-full border-b-2">
				{table.getHeaderGroups().map((headerGroup) => (
					<tr key={headerGroup.id}>
						{headerGroup.headers.map((header) => {
							// Check if column is an SNMP column (has Icon and name but uses simple header function)
							// @ts-ignore
							const columnDef = header.column.columnDef as ColumnDef<SystemRecord, unknown>
							const hasIcon = 'Icon' in columnDef && columnDef.Icon
							const hasName = 'name' in columnDef && columnDef.name
							const isSortableHeader = columnDef.header === sortableHeader
							const isSnmpColumn = hasIcon && hasName && !isSortableHeader
							
							return (
								<TableHead className="px-1.5" key={header.id}>
									{isSnmpColumn ? (
										<div className="flex items-center gap-2">
											<columnDef.Icon className="size-4 text-muted-foreground" />
											{flexRender(header.column.columnDef.header, header.getContext())}
										</div>
									) : (
										flexRender(header.column.columnDef.header, header.getContext())
									)}
								</TableHead>
							)
						})}
					</tr>
				))}
			</TableHeader>
		)
	}, [i18n.locale, colLength])
}

const SystemTableRow = memo(function ({
	row,
	virtualRow,
	colLength,
}: {
	row: Row<SystemRecord>
	virtualRow: VirtualItem
	length: number
	colLength: number
}) {
	const system = row.original
	const { t } = useLingui()
	return useMemo(() => {
		return (
			<TableRow
				// data-state={row.getIsSelected() && "selected"}
				className={cn("cursor-pointer transition-opacity relative safari:transform-3d", {
					"opacity-50": system.status === SystemStatus.Paused,
				})}
			>
				{row.getVisibleCells().map((cell) => (
					<TableCell
						key={cell.id}
						style={{
							width: cell.column.getSize(),
							height: virtualRow.size,
						}}
						className="py-0"
					>
						{flexRender(cell.column.columnDef.cell, cell.getContext())}
					</TableCell>
				))}
			</TableRow>
		)
	}, [system, system.status, colLength, t])
})

