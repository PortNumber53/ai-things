import { Composition, staticFile } from 'remotion';
import { AudioGramSchema, AudiogramComposition, fps } from './Composition';
import './style.css';

export const RemotionRoot: React.FC = () => {
	return (
		<>
			<Composition
				id="Audiogram"
				component={AudiogramComposition}
				fps={fps}
				width={1080}
				height={1920}
				schema={AudioGramSchema}
				defaultProps={{
					// Audio settings
					audioOffsetInSeconds: 0,

					// Title settings
					audioFileName: staticFile(
						'0000000005-output_combined_with_silence.mp3'
					),
					coverImgFileName: staticFile('cover.jpg'),
					titleText: '00005 - The Magic of Mushrooms',
					titleColor: 'rgba(186, 186, 186, 0.93)',

					// Subtitles settings
					subtitlesFileName: staticFile('subtitles.srt'),
					onlyDisplayCurrentSentence: true,
					subtitlesTextColor: 'rgba(255, 255, 255, 0.93)',
					subtitlesLinePerPage: 4,
					subtitlesZoomMeasurerSize: 10,
					subtitlesLineHeight: 98,

					// Wave settings
					waveColor: '#93d5ae',
					waveFreqRangeStartIndex: 7,
					waveLinesToDisplay: 60,
					waveNumberOfSamples: '256', // This is string for Remotion controls and will be converted to a number
					mirrorWave: false,
					durationInSeconds: 169.5,
				}}
				// Determine the length of the video based on the duration of the audio file
				calculateMetadata={({ props }) => {
					return {
						durationInFrames: props.durationInSeconds * fps,
						props,
					};
				}}
			/>
		</>
	);
};
