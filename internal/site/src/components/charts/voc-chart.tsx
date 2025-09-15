import { CartesianGrid, Line, LineChart, YAxis } from "recharts"

import {
	ChartContainer,
	ChartLegend,
	ChartLegendContent,
	ChartTooltip,
	ChartTooltipContent,
	xAxis,
} from "@/components/ui/chart"
import { cn, formatShortDate, chartMargin, decimalString, toFixedFloat } from "@/lib/utils"
import { ChartData } from "@/types"
import { memo, useMemo } from "react"

export default memo(function VOCChart({ chartData }: { chartData: ChartData }) {
	if (chartData.systemStats.length === 0) {
		return null
	}

	const newChartData = useMemo(() => {
		const newChartData = { data: [], colors: {} } as {
			data: Record<string, number | string>[]
			colors: Record<string, string>
		}
		const sums = {} as Record<string, number>
		for (let data of chartData.systemStats) {
			let newData = { created: data.created } as Record<string, number | string>
			let keys = Object.keys((data.stats as any)?.voc ?? {})
			for (let i = 0; i < keys.length; i++) {
				let key = keys[i]
				newData[key] = (data.stats as any).voc[key]
				sums[key] = (sums[key] ?? 0) + (newData[key] as number)
			}
			newChartData.data.push(newData)
		}
		const keys = Object.keys(sums).sort((a, b) => sums[b] - sums[a])
		for (let key of keys) {
			newChartData.colors[key] = `hsl(${((keys.indexOf(key) * 360) / keys.length) % 360}, 60%, 55%)`
		}
		return newChartData
	}, [chartData])

	const colors = Object.keys(newChartData.colors)

	return (
		<div>
			<ChartContainer className={cn("h-full w-full absolute aspect-auto bg-card opacity-0 transition-opacity", { "opacity-100": true })}>
				<LineChart accessibilityLayer data={newChartData.data} margin={chartMargin}>
					<CartesianGrid vertical={false} />
					<YAxis
						direction="ltr"
						orientation={chartData.orientation}
						className="tracking-tighter"
						width={56}
						domain={[0, "auto"]}
						tickFormatter={(val: number) => `${toFixedFloat(val, 0)} ppb`}
						tickLine={false}
						axisLine={false}
					/>
					{xAxis(chartData)}
					<ChartTooltip
						animationEasing="ease-out"
						animationDuration={150}
						// @ts-ignore
						itemSorter={(a: any, b: any) => b.value - a.value}
						content={
							<ChartTooltipContent
								labelFormatter={(_: any, data: any[]) => formatShortDate(data[0].payload.created)}
								contentFormatter={(item: any) => `${decimalString(item.value)} ppb`}
							/>
						}
					/>
					{colors.map((key) => (
						<Line key={key} dataKey={key} name={key} type="monotoneX" dot={false} strokeWidth={1.5} stroke={newChartData.colors[key]} isAnimationActive={false} />
					))}
					{colors.length < 12 && <ChartLegend content={<ChartLegendContent />} />}
				</LineChart>
			</ChartContainer>
		</div>
	)
})
