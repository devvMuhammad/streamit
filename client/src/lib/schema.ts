import { z } from 'zod';

export const startStreamSchema = z.object({
	title: z.string().min(10, 'Title must be at least 10 characters long'),
	description: z.string().min(1, 'Description must be at least 10 characters long'),
	tags: z.array(z.string()).min(1, 'You must provide at least one tag')
});

export type TStartStreamSchema = z.infer<typeof startStreamSchema>;
