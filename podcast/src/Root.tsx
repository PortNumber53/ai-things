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
					audioFileName: staticFile('output_combined_with_silence_182.mp3'),
					coverImgFileName: staticFile(
						'Default_Spaghettification_The_Bizarre_Phenomenon_in_Black_Hole_0.jpg'
					),
					titleText:
						'00182 - Spaghettification: The Bizarre Phenomenon in Black Holes',
					titleColor: 'rgba(186, 186, 186, 0.93)',

					// Subtitles settings
					subtitlesFileName: staticFile('subtitles.srt'),
					onlyDisplayCurrentSentence: true,
					subtitlesTextColor: 'rgba(255, 255, 255, 0.93)',
					subtitlesLinePerPage: 4,
					subtitlesZoomMeasurerSize: 10,
					subtitlesLineHeight: 90,

					// Wave settings
					waveColor: '#93d5ae',
					waveFreqRangeStartIndex: 7,
					waveLinesToDisplay: 40,
					waveNumberOfSamples: '256', // This is string for Remotion controls and will be converted to a number
					mirrorWave: false,
					durationInSeconds: 107.5,
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
